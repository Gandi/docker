#include "libzfs_wrapper.h"

#define SYM(result, name, args_type, args_params, args_list) \
	result (_ ## name) args_params { \
		return __ ## name args_list; \
	}
	/*
	 *void _foo (a int) {
	 *  return __foo(a);
	 *}
	 */

LIBZFS_SYMS
LIBNVPAIR_SYMS
#undef SYM

char *load_libzfs() {
	void *libzfs_hdl, *libnvpair_hdl;
	char *error;

	libzfs_hdl = dlopen("libzfs.so", RTLD_LAZY);

	if (libzfs_hdl == NULL) {
		return dlerror();
	}
	libnvpair_hdl = dlopen("libnvpair.so", RTLD_LAZY);
	if (libnvpair_hdl == NULL) {
		error = dlerror();
		dlclose(libzfs_hdl);
		return error;
	}

#define SYM(result, name, args_type, args_params, args_list) \
	__ ## name = (result (*) args_type) dlsym(libzfs_hdl, #name); \
	if (__ ## name == NULL) goto error;
	/*
	 *__foo = (void (*) (int)) dlsym(libzfs_hdl, "foo");
	 *if (__foo == NULL) goto error;
	 */
	LIBZFS_SYMS
#undef SYM

#define SYM(result, name, args_type, args_params, args_list) \
		__ ## name = (result (*) args_type) dlsym(libnvpair_hdl, #name); \
		if (__ ## name == NULL) goto error;
		LIBNVPAIR_SYMS
#undef SYM

	return NULL;
error:
	error = dlerror();
	dlclose(libzfs_hdl);
	dlclose(libnvpair_hdl);
	return error;
}
