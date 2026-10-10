#pragma once
#import "GSPhotosCompatibility.h"
#import <objc/message.h>

// Private getters must match the complete ABI before objc_msgSend is cast.
static inline BOOL GSPhotosObjectHasMethod(id object, NSString *name, const char *encoding) {
    return GSPhotosHasMethod(object_getClass(object), name, encoding);
}

static inline id GSPhotosGetObject(id object, NSString *name) {
    if (!GSPhotosObjectHasMethod(object, name, "@16@0:8"))
        return nil;
    return ((id(*)(id, SEL))objc_msgSend)(object, NSSelectorFromString(name));
}
