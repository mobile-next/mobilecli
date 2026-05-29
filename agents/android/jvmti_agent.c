#include <jvmti.h>
#include <jni.h>
#include <android/log.h>
#include <string.h>
#include <stdlib.h>

#define TAG         "devicekit"
#define AGENT_CLASS "com.mobilenext.mobilecli.MobilecliAgent"
#define LOG(...)  __android_log_print(ANDROID_LOG_DEBUG, TAG, __VA_ARGS__)

/* ── load devicekit.dex into the target process ─────────────────────────── */

/* dex_path  — full path to devicekit.dex on the device
   opt_dir   — directory used by DexClassLoader for optimised odex output;
               typically the same directory that contains the dex */
static void bootstrap(JNIEnv *env, const char *dex_path, const char *opt_dir) {
    jclass cls_CL = (*env)->FindClass(env, "java/lang/ClassLoader");
    jclass cls_DCL = (*env)->FindClass(env, "dalvik/system/DexClassLoader");
    if (!cls_CL || !cls_DCL || (*env)->ExceptionCheck(env)) {
        (*env)->ExceptionClear(env);
        return;
    }

    jmethodID getSys = (*env)->GetStaticMethodID(env, cls_CL, "getSystemClassLoader", "()Ljava/lang/ClassLoader;");
    jmethodID init = (*env)->GetMethodID(env, cls_DCL, "<init>", "(Ljava/lang/String;Ljava/lang/String;Ljava/lang/String;Ljava/lang/ClassLoader;)V");
    jmethodID load = (*env)->GetMethodID(env, cls_CL, "loadClass", "(Ljava/lang/String;)Ljava/lang/Class;");
    if (!getSys || !init || !load) {
        (*env)->ExceptionClear(env);
        return;
    }

    jobject parent = (*env)->CallStaticObjectMethod(env, cls_CL, getSys);
    jstring j_dex = (*env)->NewStringUTF(env, dex_path);
    jstring j_opt = (*env)->NewStringUTF(env, opt_dir);
    jobject loader = (*env)->NewObject(env, cls_DCL, init, j_dex, j_opt, NULL, parent);
    (*env)->DeleteLocalRef(env, j_dex);
    (*env)->DeleteLocalRef(env, j_opt);

    if (!loader || (*env)->ExceptionCheck(env)) {
        jthrowable exc = (*env)->ExceptionOccurred(env);
        (*env)->ExceptionClear(env);
        if (exc) {
            jclass ecls = (*env)->GetObjectClass(env, exc);
            jmethodID toStr = (*env)->GetMethodID(env, ecls, "toString", "()Ljava/lang/String;");
            if (toStr) {
                jstring msg = (jstring)(*env)->CallObjectMethod(env, exc, toStr);
                if (msg && !(*env)->ExceptionCheck(env)) {
                    const char *s = (*env)->GetStringUTFChars(env, msg, NULL);
                    LOG("DexClassLoader exception: %s", s ? s : "(null)");
                    if (s) (*env)->ReleaseStringUTFChars(env, msg, s);
                } else (*env)->ExceptionClear(env);
            }
        }
        LOG("DexClassLoader failed — push devicekit.dex to %s", dex_path);
        return;
    }

    jstring j_cls = (*env)->NewStringUTF(env, AGENT_CLASS);
    jclass agentCls = (jclass)(*env)->CallObjectMethod(env, loader, load, j_cls);
    (*env)->DeleteLocalRef(env, j_cls);
    if (!agentCls || (*env)->ExceptionCheck(env)) {
        jthrowable exc = (*env)->ExceptionOccurred(env);
        (*env)->ExceptionClear(env);
        if (exc) {
            jclass ecls = (*env)->GetObjectClass(env, exc);
            jmethodID toStr = (*env)->GetMethodID(env, ecls, "toString", "()Ljava/lang/String;");
            if (toStr) {
                jstring msg = (jstring)(*env)->CallObjectMethod(env, exc, toStr);
                if (msg && !(*env)->ExceptionCheck(env)) {
                    const char *s = (*env)->GetStringUTFChars(env, msg, NULL);
                    LOG("loadClass exception: %s", s ? s : "(null)");
                    if (s) (*env)->ReleaseStringUTFChars(env, msg, s);
                } else {
                    (*env)->ExceptionClear(env);
                }
            }
        }

        LOG("loadClass(MobilecliAgent) failed");
        return;
    }

    jmethodID start = (*env)->GetStaticMethodID(env, agentCls, "start", "()V");
    if (!start || (*env)->ExceptionCheck(env)) {
        (*env)->ExceptionClear(env);
        return;
    }

    (*env)->CallStaticVoidMethod(env, agentCls, start);
    if ((*env)->ExceptionCheck(env)) {
        jthrowable exc = (*env)->ExceptionOccurred(env);
        (*env)->ExceptionClear(env);
        if (exc) {
            jclass ecls = (*env)->GetObjectClass(env, exc);
            jmethodID toStr = (*env)->GetMethodID(env, ecls, "toString", "()Ljava/lang/String;");
            if (toStr) {
                jstring msg = (jstring)(*env)->CallObjectMethod(env, exc, toStr);
                if (msg && !(*env)->ExceptionCheck(env)) {
                    const char *s = (*env)->GetStringUTFChars(env, msg, NULL);
                    LOG("MobilecliAgent.start() threw: %s", s ? s : "(null)");
                    if (s) (*env)->ReleaseStringUTFChars(env, msg, s);
                } else {
                    (*env)->ExceptionClear(env);
                }
            }
        }
    }

    LOG("MobilecliAgent started");
}

/* ── agent entry points ──────────────────────────────────────────────────── */

/* opts is the dex path passed via: am attach-agent <pid> agent.so=<dex_path>
   opt_dir is derived as the directory containing the dex file. */
static jint setup(JavaVM *vm, const char *opts) {
    if (!opts || opts[0] == '\0') {
        LOG("no dex path provided — pass it as agent.so=/path/to/devicekit.dex");
        return JNI_ERR;
    }

    const char *dex_path = opts;

    /* derive opt_dir as dirname(dex_path) */
    char opt_dir[512];
    const char *slash = strrchr(dex_path, '/');
    if (slash && slash != dex_path) {
        size_t len = (size_t)(slash - dex_path);
        if (len >= sizeof(opt_dir)) {
            LOG("dex path too long (%zu bytes, max %zu): %s", len, sizeof(opt_dir) - 1, dex_path);
            return JNI_ERR;
        }
        memcpy(opt_dir, dex_path, len);
        opt_dir[len] = '\0';
    } else {
        opt_dir[0] = '.';
        opt_dir[1] = '\0';
    }

    JNIEnv *env = NULL;
    (*vm)->AttachCurrentThread(vm, (void **) &env, NULL);
    bootstrap(env, dex_path, opt_dir);
    LOG("agent ready (dex=%s)", dex_path);
    return JNI_OK;
}

JNIEXPORT jint
JNICALL Agent_OnLoad(JavaVM *vm, char *opts, void *reserved) {
    return setup(vm, opts);
}

JNIEXPORT jint
JNICALL Agent_OnAttach(JavaVM *vm, char *opts, void *reserved) {
    return setup(vm, opts);
}
