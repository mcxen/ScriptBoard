/*
 * JNI bridge between Kotlin (com.demo.scriptwidgets.engine.MujsNative) and MuJS.
 *
 * Design notes:
 *  - One js_State per engine instance. Kotlin owns the handle and must call
 *    nativeFree() exactly once. Never call into one handle from two threads.
 *  - Errors never throw Java exceptions. nativeEval() returns null on success or
 *    a JSON error string; nativeCall() always returns a string (the JS side
 *    is expected to return JSON, and C-level failures are wrapped as JSON too).
 *  - Resource limits (instruction count and heap size) keep a bad user script
 *    from freezing the widget updater forever.
 */
#include <jni.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "mujs/mujs.h"

#define RUN_LIMIT 10000000          /* instruction budget per engine (approx.) */
#define MEM_LIMIT (32 * 1024 * 1024) /* bytes */

#define JNI_PREFIX(name) Java_com_demo_scriptwidgets_engine_MujsNative_##name

/* Builds {"ok":false,"error":"<escaped msg>"} as a Java string. */
static jstring make_error_json(JNIEnv *env, const char *msg) {
    size_t n = strlen(msg);
    char *buf = (char *)malloc(n * 6 + 64);
    if (!buf) {
        return (*env)->NewStringUTF(env, "{\"ok\":false,\"error\":\"out of memory\"}");
    }
    char *p = buf;
    p += sprintf(p, "{\"ok\":false,\"error\":\"");
    for (const unsigned char *s = (const unsigned char *)msg; *s; s++) {
        switch (*s) {
        case '"':  *p++ = '\\'; *p++ = '"';  break;
        case '\\': *p++ = '\\'; *p++ = '\\'; break;
        case '\n': *p++ = '\\'; *p++ = 'n';  break;
        case '\r': *p++ = '\\'; *p++ = 'r';  break;
        case '\t': *p++ = '\\'; *p++ = 't';  break;
        default:
            if (*s < 0x20) p += sprintf(p, "\\u%04x", *s);
            else *p++ = (char)*s;
        }
    }
    p += sprintf(p, "\"}");
    *p = '\0';
    jstring out = (*env)->NewStringUTF(env, buf);
    free(buf);
    return out;
}

/* Converts the exception object on top of the MuJS stack into an error JSON and pops it. */
static jstring error_from_stack(JNIEnv *env, js_State *J, const char *fallback) {
    const char *msg = js_trystring(J, -1, fallback);
    jstring out = make_error_json(env, msg);
    js_pop(J, 1);
    return out;
}

JNIEXPORT jlong JNICALL
JNI_PREFIX(nativeNew)(JNIEnv *env, jclass clazz) {
    js_State *J = js_newstate(NULL, NULL, 0);
    if (!J) return 0;
    js_setlimit(J, RUN_LIMIT, MEM_LIMIT);
    return (jlong)(intptr_t)J;
}

JNIEXPORT void JNICALL
JNI_PREFIX(nativeFree)(JNIEnv *env, jclass clazz, jlong handle) {
    js_State *J = (js_State *)(intptr_t)handle;
    if (J) js_freestate(J);
}

/* Compiles and runs `source` in the global scope. Returns null on success. */
JNIEXPORT jstring JNICALL
JNI_PREFIX(nativeEval)(JNIEnv *env, jclass clazz, jlong handle, jstring source) {
    js_State *J = (js_State *)(intptr_t)handle;
    const char *src = (*env)->GetStringUTFChars(env, source, NULL);
    jstring result = NULL;
    if (!src) return make_error_json(env, "cannot read script source");

    if (js_ploadstring(J, "[script]", src) != 0) {
        result = error_from_stack(env, J, "syntax error");
    } else {
        js_pushundefined(J);            /* this */
        if (js_pcall(J, 0) != 0) {
            result = error_from_stack(env, J, "runtime error");
        } else {
            js_pop(J, 1);               /* discard completion value */
        }
    }

    (*env)->ReleaseStringUTFChars(env, source, src);
    return result;
}

/*
 * Calls global function `fnName(arg)` where `arg` is a string and returns the
 * string the function returned (expected to be JSON; the Kotlin side's prelude
 * always returns JSON).
 */
JNIEXPORT jstring JNICALL
JNI_PREFIX(nativeCall)(JNIEnv *env, jclass clazz, jlong handle, jstring fnName, jstring arg) {
    js_State *J = (js_State *)(intptr_t)handle;
    const char *name = (*env)->GetStringUTFChars(env, fnName, NULL);
    const char *argStr = (*env)->GetStringUTFChars(env, arg, NULL);
    jstring result;

    if (!name || !argStr) {
        if (name) (*env)->ReleaseStringUTFChars(env, fnName, name);
        if (argStr) (*env)->ReleaseStringUTFChars(env, arg, argStr);
        return make_error_json(env, "cannot read call arguments");
    }

    js_getglobal(J, name);              /* function */
    js_pushundefined(J);                /* this */
    js_pushstring(J, argStr);           /* argument */
    if (js_pcall(J, 1) != 0) {
        result = error_from_stack(env, J, "call failed");
    } else {
        const char *out = js_tostring(J, -1);
        result = (*env)->NewStringUTF(env, out ? out : "null");
        js_pop(J, 1);
    }

    (*env)->ReleaseStringUTFChars(env, fnName, name);
    (*env)->ReleaseStringUTFChars(env, arg, argStr);
    return result;
}
