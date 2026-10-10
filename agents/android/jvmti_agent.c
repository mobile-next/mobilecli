#include <jvmti.h>
#include <jni.h>
#include <android/log.h>
#include <string.h>
#include <stdlib.h>

#define TAG         "devicekit"
#define AGENT_CLASS "com.mobilenext.mobilecli.InjectedAgent"
#define LOG(...)  __android_log_print(ANDROID_LOG_DEBUG, TAG, __VA_ARGS__)

/* A breakpoint at OkHttpClient.interceptors() lets the agent replace the list
   on both clients made before attach and clients made later. The getter then
   returns that same field, with our interceptor appended once. */
static jvmtiEnv *network_jvmti;
static jmethodID network_getter;
static jfieldID network_field;
static jobject network_interceptor;

static void JNICALL network_breakpoint(jvmtiEnv *jvmti, JNIEnv *env,
        jthread thread, jmethodID method, jlocation location) {
    (void) location;
    if (method != network_getter || network_interceptor == NULL) return;
    jobject client = NULL;
    jvmtiError local_err = (*jvmti)->GetLocalInstance(jvmti, thread, 0, &client);
    if (local_err != JVMTI_ERROR_NONE || !client) {
        LOG("OkHttp getter receiver unavailable: %d", local_err);
        return;
    }
    jobject old_list = (*env)->GetObjectField(env, client, network_field);
    if (!old_list || (*env)->ExceptionCheck(env)) goto done;

    jclass list_cls = (*env)->FindClass(env, "java/util/List");
    jmethodID contains = (*env)->GetMethodID(env, list_cls, "contains", "(Ljava/lang/Object;)Z");
    if ((*env)->CallBooleanMethod(env, old_list, contains, network_interceptor)) goto done;
    if ((*env)->ExceptionCheck(env)) goto done;

    jclass array_cls = (*env)->FindClass(env, "java/util/ArrayList");
    jmethodID init = (*env)->GetMethodID(env, array_cls, "<init>", "(Ljava/util/Collection;)V");
    jobject new_list = (*env)->NewObject(env, array_cls, init, old_list);
    jmethodID add = (*env)->GetMethodID(env, array_cls, "add", "(Ljava/lang/Object;)Z");
    (*env)->CallBooleanMethod(env, new_list, add, network_interceptor);
    if (!(*env)->ExceptionCheck(env)) {
        (*env)->SetObjectField(env, client, network_field, new_list);
    }
    (*env)->DeleteLocalRef(env, new_list);
done:
    if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
    if (old_list) (*env)->DeleteLocalRef(env, old_list);
    (*env)->DeleteLocalRef(env, client);
}

static jboolean JNICALL network_install(JNIEnv *env, jclass unused, jclass client_cls, jobject interceptor) {
    (void) unused;
    if (!network_jvmti) return JNI_FALSE;
    jmethodID getter = (*env)->GetMethodID(env, client_cls, "interceptors", "()Ljava/util/List;");
    jfieldID field = (*env)->GetFieldID(env, client_cls, "interceptors", "Ljava/util/List;");
    if (!getter || !field || (*env)->ExceptionCheck(env)) return JNI_FALSE;
    jvmtiCapabilities caps = {0};
    caps.can_generate_breakpoint_events = 1;
    caps.can_access_local_variables = 1;
    jvmtiError err = (*network_jvmti)->AddCapabilities(network_jvmti, &caps);
    if (err != JVMTI_ERROR_NONE) { LOG("network JVMTI capabilities failed: %d", err); return JNI_FALSE; }
    if (network_getter) (*network_jvmti)->ClearBreakpoint(network_jvmti, network_getter, 0);
    if (network_interceptor) (*env)->DeleteGlobalRef(env, network_interceptor);
    network_interceptor = (*env)->NewGlobalRef(env, interceptor);
    network_getter = getter;
    network_field = field;
    jvmtiEventCallbacks callbacks = {0};
    callbacks.Breakpoint = network_breakpoint;
    err = (*network_jvmti)->SetEventCallbacks(network_jvmti, &callbacks, sizeof(callbacks));
    if (err == JVMTI_ERROR_NONE) err = (*network_jvmti)->SetBreakpoint(network_jvmti, getter, 0);
    if (err == JVMTI_ERROR_NONE) err = (*network_jvmti)->SetEventNotificationMode(
        network_jvmti, JVMTI_ENABLE, JVMTI_EVENT_BREAKPOINT, NULL);
    if (err != JVMTI_ERROR_NONE) {
        LOG("OkHttp getter breakpoint failed: %d", err);
        return JNI_FALSE;
    }
    return JNI_TRUE;
}

/* ── load mobilecli.dex into the target process ─────────────────────────── */

/* dex_path  — full path to mobilecli.dex on the device
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
        LOG("DexClassLoader failed — push mobilecli.dex to %s", dex_path);
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

        LOG("loadClass(InjectedAgent) failed");
        return;
    }

    jstring j_hook = (*env)->NewStringUTF(env, "com.mobilenext.mobilecli.OkHttpHook");
    jclass hook_cls = (jclass)(*env)->CallObjectMethod(env, loader, load, j_hook);
    (*env)->DeleteLocalRef(env, j_hook);
    if (hook_cls && !(*env)->ExceptionCheck(env)) {
        JNINativeMethod methods[] = {{"nativeInstall", "(Ljava/lang/Class;Ljava/lang/Object;)Z", (void *)network_install}};
        if ((*env)->RegisterNatives(env, hook_cls, methods, 1) != JNI_OK) (*env)->ExceptionClear(env);
    } else (*env)->ExceptionClear(env);

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
                    LOG("InjectedAgent.start() threw: %s", s ? s : "(null)");
                    if (s) (*env)->ReleaseStringUTFChars(env, msg, s);
                } else {
                    (*env)->ExceptionClear(env);
                }
            }
        }
    }

    LOG("InjectedAgent started");
}

/* ── agent entry points ──────────────────────────────────────────────────── */

/* opts is the dex path passed via: am attach-agent <pid> agent.so=<dex_path>
   opt_dir is derived as the directory containing the dex file. */
static jint setup(JavaVM *vm, const char *opts) {
    if (!opts || opts[0] == '\0') {
        LOG("no dex path provided — pass it as agent.so=/path/to/mobilecli.dex");
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

    if ((*vm)->GetEnv(vm, (void **)&network_jvmti, JVMTI_VERSION_1_2) != JNI_OK) {
        LOG("JVMTI environment unavailable");
        return JNI_ERR;
    }
    jint rc = (*vm)->AttachCurrentThread(vm, (void **) &env, NULL);
    if (rc != JNI_OK || env == NULL) {
        LOG("AttachCurrentThread failed (rc=%d, dex=%s)", rc, dex_path);
        return JNI_ERR;
    }
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
