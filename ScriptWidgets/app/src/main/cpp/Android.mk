LOCAL_PATH := $(call my-dir)

include $(CLEAR_VARS)

LOCAL_MODULE    := scriptengine
# mujs/one.c is the MuJS amalgamation (ISC license, see mujs/COPYING.mujs).
LOCAL_SRC_FILES := jni_bridge.c mujs/one.c
LOCAL_C_INCLUDES := $(LOCAL_PATH)/mujs
LOCAL_CFLAGS    := -O2 -DNDEBUG -Wall -Wno-unused-parameter
LOCAL_LDLIBS    := -llog -lm

include $(BUILD_SHARED_LIBRARY)
