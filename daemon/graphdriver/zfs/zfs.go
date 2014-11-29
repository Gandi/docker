package zfs

/*
#include <locale.h>
#include <stdlib.h>
#include <dirent.h>
#include <mntent.h>
*/
import "C"

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"syscall"
	"time"
	"unsafe"

	zfs "github.com/Mic92/go-zfs"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/parsers"
)

type ZfsOptions struct {
	fsName    string
	mountPath string
}

func init() {
	graphdriver.Register("zfs", Init)
}

type Logger struct{}

func (*Logger) Log(cmd []string) {
	log.Debugf("[zfs] %s", strings.Join(cmd, " "))
}

func Init(base string, opt []string) (graphdriver.Driver, error) {
	var err error
	options, err := parseOptions(opt)
	options.mountPath = base
	if err != nil {
		return nil, err
	}

	rootdir := path.Dir(base)

	if options.fsName == "" {
		err = checkRootdirFs(rootdir)
		if err != nil {
			return nil, err
		}
	}

	if _, err := exec.LookPath("zfs"); err != nil {
		return nil, fmt.Errorf("zfs command is not available")
	}

	file, err := os.OpenFile("/dev/zfs", os.O_RDWR, 600)
	defer file.Close()
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize: %v", err)
	}

	if options.fsName == "" {
		options.fsName, err = lookupZfsDataset(rootdir)
		if err != nil {
			return nil, err
		}
	}

	logger := Logger{}
	zfs.SetLogger(&logger)

	dataset, err := zfs.GetDataset(options.fsName)
	if err != nil {
		return nil, fmt.Errorf("Cannot open %s", options.fsName)
	}

	return &Driver{
		dataset: dataset,
		options: options,
	}, nil
}

func parseOptions(opt []string) (ZfsOptions, error) {
	var options ZfsOptions
	options.fsName = ""
	for _, option := range opt {
		key, val, err := parsers.ParseKeyValueOpt(option)
		if err != nil {
			return options, err
		}
		key = strings.ToLower(key)
		switch key {
		case "zfs.fsname":
			options.fsName = val
		default:
			return options, fmt.Errorf("Unknown option %s\n", key)
		}
	}
	return options, nil
}

func checkRootdirFs(rootdir string) error {
	var buf syscall.Statfs_t
	if err := syscall.Statfs(rootdir, &buf); err != nil {
		fmt.Errorf("Failed to access '%s': %s", rootdir, err)
	}

	if graphdriver.FsMagic(buf.Type) != graphdriver.FsMagicZfs {
		log.Debugf("[zfs] no zfs dataset found for rootdir '%s'", rootdir)
		return graphdriver.ErrPrerequisites
	}
	return nil
}

var CprocMounts = C.CString("/proc/mounts")
var CopenMod = C.CString("r")

func lookupZfsDataset(rootdir string) (string, error) {
	var stat syscall.Stat_t
	var Cmnt C.struct_mntent
	var Cfp *C.FILE
	buf := string(make([]byte, 256, 256))
	Cbuf := C.CString(buf)
	defer free(Cbuf)

	if err := syscall.Stat(rootdir, &stat); err != nil {
		return "", fmt.Errorf("Failed to access '%s': %s", rootdir, err)
	}
	wantedDev := stat.Dev

	if Cfp = C.setmntent(CprocMounts, CopenMod); Cfp == nil {
		return "", fmt.Errorf("Failed to open /proc/mounts")
	}
	defer C.endmntent(Cfp)

	for C.getmntent_r(Cfp, &Cmnt, Cbuf, 256) != nil {
		dir := C.GoString(Cmnt.mnt_dir)
		if err := syscall.Stat(dir, &stat); err != nil {
			log.Debugf("[zfs] failed to stat '%s' while scanning for zfs mount: %v", dir, err)
			continue // may fail on fuse file systems
		}

		fs := C.GoString(Cmnt.mnt_type)
		if stat.Dev == wantedDev && fs == "zfs" {
			return C.GoString(Cmnt.mnt_fsname), nil
		}
	}
	// should never happen
	return "", fmt.Errorf("Failed to find zfs pool in /proc/mounts")
}

func free(p *C.char) {
	C.free(unsafe.Pointer(p))
}

type Driver struct {
	dataset *zfs.Dataset
	options ZfsOptions
}

func (d *Driver) String() string {
	return "zfs"
}

func (d *Driver) Cleanup() error {
	return nil
}

func (d *Driver) Status() [][2]string {
	return nil
}

func cloneFilesystem(id, parent, mountpoint string) error {
	parentDataset, err := zfs.GetDataset(parent)
	if parentDataset == nil {
		return err
	}
	snapshotName := fmt.Sprintf("%d", time.Now().Nanosecond())
	snapshot, err := parentDataset.Snapshot(snapshotName /*recursive */, false)
	if snapshot == nil {
		return err
	}

	_, err = snapshot.Clone(id, map[string]string{
		"mountpoint": mountpoint,
	})
	if err != nil {
		snapshot.Destroy( /*recursive*/ false /*deferred*/, true)
		return err
	}
	err = snapshot.Destroy( /*recursive*/ false /*deferred*/, true)
	return err
}

func (d *Driver) ZfsPath(id string) string {
	return d.options.fsName + "/" + id
}

func (d *Driver) Create(id string, parent string) error {
	mountPoint := path.Join(d.options.mountPath, "graph", id)
	if parent == "" {
		_, err := zfs.CreateFilesystem(d.ZfsPath(id), map[string]string{
			"mountpoint": mountPoint,
		})
		return err
	} else {
		return cloneFilesystem(d.ZfsPath(id), d.ZfsPath(parent), mountPoint)
	}
	return nil
}

func (d *Driver) Remove(id string) error {
	dataset, err := zfs.GetDataset(d.ZfsPath(id))
	if dataset == nil {
		return err
	}

	return dataset.Destroy( /* recursive */ true /* deferred */, false)
}

func (d *Driver) Get(id, mountLabel string) (string, error) {
	dataset, err := zfs.GetDataset(d.ZfsPath(id))
	if dataset == nil {
		return "", err
	} else {
		return dataset.Mountpoint, nil
	}
}

func (d *Driver) Put(id string) {
	// FS is already mounted
}

func (d *Driver) Exists(id string) bool {
	_, err := zfs.GetDataset(d.ZfsPath(id))
	return err != nil
}

func zfsChanges(dataset *zfs.Dataset) ([]archive.Change, error) {
	if dataset.Origin == "" { // should never happen
		return nil, fmt.Errorf("no origin found for dataset '%s'. expected a clone", dataset.Name)
	}
	changes, err := dataset.Diff(dataset.Origin)
	if err != nil {
		return nil, err
	}

	// for rename changes, we have to add a ADD and a REMOVE
	renameCount := 0
	for _, change := range changes {
		if change.Change == zfs.RENAMED {
			renameCount++
		}
	}
	archiveChanges := make([]archive.Change, len(changes)+renameCount)
	i := 0
	for _, change := range changes {
		var changeType archive.ChangeType
		mountpointLen := len(dataset.Mountpoint)
		basePath := change.Path[mountpointLen:]
		switch change.Change {
		case zfs.RENAMED:
			archiveChanges[i] = archive.Change{basePath, archive.ChangeDelete}
			newBasePath := change.NewPath[mountpointLen:]
			archiveChanges[i+1] = archive.Change{newBasePath, archive.ChangeAdd}
			i += 2
			continue
		case zfs.CREATED:
			changeType = archive.ChangeAdd
		case zfs.MODIFIED:
			changeType = archive.ChangeModify
		case zfs.REMOVED:
			changeType = archive.ChangeDelete
		}
		archiveChanges[i] = archive.Change{basePath, changeType}
		i++
	}

	return archiveChanges, nil
}

func (d *Driver) Diff(id, parent string) (archive.Archive, error) {
	dataset, err := zfs.GetDataset(d.ZfsPath(id))
	if err != nil {
		return nil, err
	}
	changes, err := zfsChanges(dataset)
	if err != nil {
		return nil, err
	}

	archive, err := archive.ExportChanges(dataset.Mountpoint, changes)
	if err != nil {
		return nil, err
	}
	return ioutils.NewReadCloserWrapper(archive, func() error {
		err := archive.Close()
		d.Put(id)
		return err
	}), nil
}

func (d *Driver) DiffSize(id, parent string) (bytes int64, err error) {
	dataset, err := zfs.GetDataset(d.ZfsPath(id))
	if err == nil {
		return int64((*dataset).Logicalused), nil
	} else {
		return -1, err
	}
}

func (d *Driver) Changes(id, parent string) ([]archive.Change, error) {
	dataset, err := zfs.GetDataset(d.ZfsPath(id))
	if err != nil {
		return nil, err
	}
	return zfsChanges(dataset)
}

func (d *Driver) ApplyDiff(id, parent string, diff archive.ArchiveReader) (int64, error) {
	dataset, err := zfs.GetDataset(d.ZfsPath(id))
	if err != nil {
		return -1, err
	}
	err = archive.ApplyLayer(dataset.Mountpoint, diff)
	if err != nil {
		return -1, err
	}
	updatedDataset, err := zfs.GetDataset(d.ZfsPath(id))
	if err != nil {
		return -1, err
	}
	return int64(updatedDataset.Logicalused), nil
}
