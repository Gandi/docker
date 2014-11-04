#include <libzfs.h>
#include <stdlib.h>
#include <dlfcn.h>

#define LIBZFS_SYMS \
	SYM0(libzfs_handle_t *, libzfs_init) \
	SYM1(void, libzfs_fini, libzfs_handle_t *) \
	SYM2(void, libzfs_print_on_error, libzfs_handle_t *, boolean_t) \
	SYM3(zfs_handle_t *, zfs_open, libzfs_handle_t *, const char *, int) \
	SYM1(void,  zfs_close, zfs_handle_t *) \
	SYM1(zfs_type_t, zfs_get_type, const zfs_handle_t *) \
	SYM1(const char *, zfs_get_name, const zfs_handle_t *) \
	SYM1(const char *, zfs_prop_to_name, zfs_prop_t) \
	SYM8(int, zfs_prop_get, zfs_handle_t *, zfs_prop_t, char *, size_t, zprop_source_t *, char *, size_t, boolean_t) \
	SYM4(int, zfs_iter_dependents, zfs_handle_t *, boolean_t, zfs_iter_f, void *) \
	SYM4(int, zfs_iter_snapspec, zfs_handle_t *, const char *, zfs_iter_f, void *) \
	SYM2(int, zfs_destroy, zfs_handle_t *, boolean_t) \
	SYM3(int, zfs_destroy_snaps_nvl, libzfs_handle_t *, nvlist_t *, boolean_t) \
	SYM3(int, zfs_clone, zfs_handle_t *, const char *, nvlist_t *) \
	SYM3(int, zfs_snapshot_nvl, libzfs_handle_t *, nvlist_t *, nvlist_t *) \
	SYM3(int, zfs_mount, zfs_handle_t *, const char *, int) \
	SYM3(int, zfs_unmount, zfs_handle_t *, const char *, int) \
	SYM4(int, zfs_create, libzfs_handle_t *, const char *, zfs_type_t, nvlist_t *) \
	SYM1(void, zpool_close, zpool_handle_t *) \
	SYM2(zpool_handle_t *, zpool_open, libzfs_handle_t *, const char *)

#define LIBNVPAIR_SYMS \
	SYM3(int, nvlist_alloc, nvlist_t **, uint_t, int) \
	SYM1(void, nvlist_free, nvlist_t *) \
	SYM3(int, nvlist_add_string, nvlist_t *, const char *, const char *) \
	SYM0(nvlist_t *, fnvlist_alloc) \
	SYM1(void, fnvlist_free, nvlist_t *) \
	SYM2(void, fnvlist_add_boolean, nvlist_t *, const char *)

#define SYM0(result, name) SYM(result, name, (), (), ())
#define SYM1(result, name, a1) SYM(result, name, (a1), (a1 a), (a))
#define SYM2(result, name, a1, a2) SYM(result, name, (a1, a2), (a1 a, a2 b), (a, b))
#define SYM3(result, name, a1, a2, a3) SYM(result, name, (a1, a2, a3), (a1 a, a2 b, a3 c), (a, b, c))
#define SYM4(result, name, a1, a2, a3, a4) SYM(result, name, (a1, a2, a3, a4), (a1 a, a2 b, a3 c, a4 d), (a, b, c, d))
#define SYM8(result, name, a1, a2, a3, a4, a5, a6, a7, a8) \
	SYM(result, name, (a1, a2, a3, a4, a5, a6, a7, a8), (a1 a, a2 b, a3 c, a4 d, a5 e, a6 f, a7 g, a8 h), (a, b, c, d, e, f, g, h))

#define SYM(result, name, args_type, args_params, args_list) \
	static result (*__ ## name) args_type = NULL; \
	result (_ ## name) args_params;
	/*
	 * static void (*__foo) (int) = NULL;
	 * void _foo (a int);
	 */

LIBZFS_SYMS
LIBNVPAIR_SYMS
#undef SYM

char *load_libzfs();
