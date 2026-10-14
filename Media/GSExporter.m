#import "../Shared/GSLocalization.h"
#import "GSExporter.h"
#import "../Shared/IPCProtocol.h"
static NSArray<NSURL *> *GSWriteOriginalResources(PHAsset *asset, NSURL *directory,
                                                  NSError **error) {
    NSArray *resources = [PHAssetResource assetResourcesForAsset:asset];
    NSMutableArray *chosen = [NSMutableArray array];
    BOOL video = asset.mediaType == PHAssetMediaTypeVideo;
    BOOL live = (asset.mediaSubtypes & PHAssetMediaSubtypePhotoLive) != 0;
    // Adjusted assets (including camera-applied adjustments the user never
    // edited) back up their current rendition: the native fingerprint lookup
    // after a GoToHP upload only finds the full-size bytes (device 2026-10-07).
    PHAssetResource * (^pick)(PHAssetResourceType, PHAssetResourceType) =
        ^PHAssetResource *(PHAssetResourceType rendered, PHAssetResourceType base) {
            PHAssetResource *found = nil;
            for (PHAssetResource *r in resources) {
                if (r.type == rendered)
                    return r;
                if (r.type == base && !found)
                    found = r;
            }
            return found;
        };
    PHAssetResource *main = video
                                ? pick(PHAssetResourceTypeFullSizeVideo, PHAssetResourceTypeVideo)
                                : pick(PHAssetResourceTypeFullSizePhoto, PHAssetResourceTypePhoto);
    if (main)
        [chosen addObject:main];
    if (live) {
        PHAssetResource *paired =
            pick(PHAssetResourceTypeFullSizePairedVideo, PHAssetResourceTypePairedVideo);
        if (paired)
            [chosen addObject:paired];
    }
    if (!chosen.count || (live && chosen.count != 2)) {
        if (error)
            *error = [NSError errorWithDomain:@"Gunshot"
                                         code:2
                                     userInfo:@{
                                         NSLocalizedDescriptionKey :
                                             GSL(@"Original media resources are unavailable.")
                                     }];
        return nil;
    }
    // Rendered resources are named FullSizeRender.*; keep the capture name so the
    // item title and the Live Photo stem pairing stay as before.
    NSString *stem = nil;
    for (PHAssetResource *r in resources)
        if (r.type == (video ? PHAssetResourceTypeVideo : PHAssetResourceTypePhoto)) {
            stem = r.originalFilename.lastPathComponent.stringByDeletingPathExtension;
            break;
        }
    NSMutableArray *files = [NSMutableArray array];
    for (PHAssetResource *r in chosen) {
        NSString *name = r.originalFilename.lastPathComponent;
        if (stem.length && name.pathExtension.length)
            name = [stem stringByAppendingPathExtension:name.pathExtension];
        NSURL *url = [directory URLByAppendingPathComponent:name];
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
