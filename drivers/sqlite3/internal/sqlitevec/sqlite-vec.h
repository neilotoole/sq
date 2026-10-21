#ifndef SQLITE_VEC_H
#define SQLITE_VEC_H

#ifndef SQLITE_CORE
#include "sqlite3ext.h"
#else
#include "sqlite3.h"
#endif

#ifdef SQLITE_VEC_STATIC
  #define SQLITE_VEC_API __attribute__((visibility("default")))
#else
  #ifdef _WIN32
    #define SQLITE_VEC_API __declspec(dllexport)
  #else
    #define SQLITE_VEC_API __attribute__((visibility("default")))
  #endif
#endif

#define SQLITE_VEC_VERSION "v0.2.7-alpha"
// TODO rm
#define SQLITE_VEC_DATE "2026-10-05T15:28:27Z+1100"
#define SQLITE_VEC_SOURCE "5ce7421ea9f71a1ee5377d6d73cbc3794925dc03"


#define SQLITE_VEC_VERSION_MAJOR 0
#define SQLITE_VEC_VERSION_MINOR 2
#define SQLITE_VEC_VERSION_PATCH 7

#ifdef __cplusplus
extern "C" {
#endif

SQLITE_VEC_API int sqlite3_vec_init(sqlite3 *db, char **pzErrMsg,
                  const sqlite3_api_routines *pApi);

#ifdef __cplusplus
}  /* end of the 'extern "C"' block */
#endif

#endif /* ifndef SQLITE_VEC_H */
