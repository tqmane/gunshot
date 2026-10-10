#import "../Shared/GSLocalization.h"
#import "GSExporter.h"
#import "../Shared/IPCProtocol.h"

// Prefer the JPEG component of a single RAW+JPEG PhotoKit asset. RAW-only
// assets must still preserve their original bytes.
static BOOL GSResourceIsRAW(PHAssetResource *resource) {
    if (!resource)
        return NO;
    NSString *uti = resource.uniformTypeIdentifier.lowercaseString;
    NSString *extension = resource.originalFilename.pathExtension.lowercaseString;
    return [uti isEqualToString:@"com.adobe.raw-image"] ||
           [uti isEqualToString:@"public.camera-raw-image"] || [uti hasSuffix:@"-raw-image"] ||
           (extension.length && [@[
               @"dng", @"arw", @"cr2", @"cr3", @"nef", @"nrw", @"orf", @"pef", @"raf",
               @"raw", @"rw2", @"srw"
           ] containsObject:extension]);
}
static BOOL GSResourceIsJPEG(PHAssetResource *resource) {
    if (!resource)
        return NO;
    NSString *uti = resource.uniformTypeIdentifier.lowercaseString;
    NSString *extension = resource.originalFilename.pathExtension.lowercaseString;
    return [uti isEqualToString:@"public.jpeg"] ||
           (extension.length && [@[ @"jpg", @"jpeg", @"jpe" ] containsObject:extension]);
}
static NSArray<NSURL *> *GSWriteOriginalResources(PHAsset *asset, NSURL *directory,
                                                  NSError **error) {
    NSArray *resources = [PHAssetResource assetResourcesForAsset:asset];
    NSMutableArray *chosen = [NSMutableArray array];
    PHAssetResourceType type = asset.mediaType == PHAssetMediaTypeVideo ? PHAssetResourceTypeVideo
                                                                        : PHAssetResourceTypePhoto;
    PHAssetResource *primary = nil;
    for (PHAssetResource *r in resources)
        if (r.type == type) {
            primary = r;
            break;
        }
    // Depending on the camera/import path, RAW can be the primary resource
    // and its JPEG counterpart an alternatePhoto. Sending only the primary
    // in that case uploads the RAW instead of the user's JPEG.
    if (asset.mediaType == PHAssetMediaTypeImage && GSResourceIsRAW(primary))
        for (PHAssetResource *r in resources)
            if (r.type == PHAssetResourceTypeAlternatePhoto && GSResourceIsJPEG(r)) {
                primary = r;
                break;
            }
    if (primary)
        [chosen addObject:primary];
    if (asset.mediaSubtypes & PHAssetMediaSubtypePhotoLive)
        for (PHAssetResource *r in resources)
            if (r.type == PHAssetResourceTypePairedVideo) {
                [chosen addObject:r];
                break;
            }
    if (!chosen.count ||
        ((asset.mediaSubtypes & PHAssetMediaSubtypePhotoLive) && chosen.count != 2)) {
        if (error)
            *error = [NSError errorWithDomain:@"Gunshot"
                                         code:2
                                     userInfo:@{
                                         NSLocalizedDescriptionKey :
                                             GSL(@"Original media resources are unavailable.")
                                     }];
        return nil;
    }
    NSMutableArray *files = [NSMutableArray array];
    for (PHAssetResource *r in chosen) {
        NSURL *url = [directory URLByAppendingPathComponent:r.originalFilename.lastPathComponent];
        if ([NSFileManager.defaultManager fileExistsAtPath:url.path])
            return nil;
        PHAssetResourceRequestOptions *options = [PHAssetResourceRequestOptions new];
        options.networkAccessAllowed = YES;
        dispatch_semaphore_t done = dispatch_semaphore_create(0);
        __block NSError *exportError = nil;
        [PHAssetResourceManager.defaultManager writeDataForAssetResource:r
                                                                  toFile:url
                                                                 options:options
                                                       completionHandler:^(NSError *e) {
                                                           exportError = e;
                                                           dispatch_semaphore_signal(done);
                                                       }];
        dispatch_semaphore_wait(done, DISPATCH_TIME_FOREVER);
        if (exportError) {
            if (error)
                *error = exportError;
            return nil;
        }
        [files addObject:url];
    }
    return files;
}
NSArray<NSURL *> *GSExportAsset(PHAsset *asset, NSURL *directory, NSError **error) {
    // Native backup may schedule many assets simultaneously. Keep PhotoKit/cloud
    // resource preparation bounded across native, album, picker and share routes;
    // this does not change the queue's network upload concurrency.
    NSCAssert(!NSThread.isMainThread, @"Export originals on a worker");
    static dispatch_queue_t exports;
    static dispatch_once_t once;
    dispatch_once(&once, ^{
        exports =
            dispatch_queue_create("dev.tqmane.gunshot.original-export", DISPATCH_QUEUE_SERIAL);
    });
    __block NSArray *files = nil;
    __block NSError *failure = nil;
    dispatch_sync(exports, ^{
        @autoreleasepool {
            files = GSWriteOriginalResources(asset, directory, &failure);
        }
    });
    if (error)
        *error = failure;
    return files;
}
NSString *GSImportFiles(NSArray<NSURL *> *files, NSString *account, NSString *quality, NSDate *date,
                        NSError **error) {
    NSMutableArray *resources = [NSMutableArray array];
    for (NSURL *u in files) {
        NSDictionary *attrs = [NSFileManager.defaultManager attributesOfItemAtPath:u.path
                                                                             error:error];
        if (!attrs || ![attrs[NSFileType] isEqual:NSFileTypeRegular])
            return nil;
        [resources addObject:@{@"name" : u.lastPathComponent, @"size" : attrs[NSFileSize]}];
    }
    NSDictionary *begin = GSRequest(
        @{
            @"op" : @"begin",
            @"account" : account ?: @"",
            @"quality" : quality ?: @"original",
            @"timestamp" : @((long long)(date ?: NSDate.date).timeIntervalSince1970),
            @"resources" : resources
        },
        error);
    NSString *identifier = begin[@"id"];
    if (!identifier)
        return nil;
    BOOL success = NO;
    @try {
        for (NSUInteger i = 0; i < files.count; i++) {
            NSFileHandle *f = [NSFileHandle fileHandleForReadingAtPath:files[i].path];
            if (!f)
                return nil;
            @try {
                unsigned long long offset = 0;
                while (YES) {
                    @autoreleasepool {
                        NSData *chunk = [f readDataUpToLength:32768 error:error];
                        if (!chunk)
                            return nil;
                        if (!chunk.length)
                            break;
                        if (!GSRequest(
                                @{
                                    @"op" : @"append",
                                    @"id" : identifier,
                                    @"index" : @(i),
                                    @"offset" : @(offset),
                                    @"data" : [chunk base64EncodedStringWithOptions:0]
                                },
                                error))
                            return nil;
                        offset += chunk.length;
                    }
                }
            } @finally {
                [f closeAndReturnError:nil];
            }
        }
        NSDictionary *sealed = GSRequest(@{@"op" : @"seal", @"id" : identifier}, error);
        success = sealed != nil;
        return sealed[@"id"];
    } @finally {
        if (!success)
            GSRequest(@{@"op" : @"cancel", @"id" : identifier}, nil);
    }
}
