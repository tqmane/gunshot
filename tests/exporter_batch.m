#import "../Media/GSBatchImport.h"
#import "../Media/GSExporter.h"
#include <assert.h>
#include <stdatomic.h>

// Exercise the real PhotoKit exporter, 32 KiB IPC importer and batch worker.
// Opaque bytes stand in for PhotoKit originals; no codec or network is mocked
// as successfully decoding these bytes.
static NSUInteger Queued, Written;
static BOOL IncludeUnreadable;
static atomic_int ActiveExports, PeakExports;
static NSArray *ExpectedResources;
static NSMutableArray<NSMutableData *> *Received;
static NSData *OriginalBytes(BOOL movie) {
    NSMutableData *bytes = [NSMutableData dataWithLength:70013];
    uint8_t *p = bytes.mutableBytes;
    for (NSUInteger i = 0; i < bytes.length; i++)
        p[i] = (uint8_t)(i * 17 + (movie ? 3 : 7));
    memcpy(p,
           "\0\0\0\x18"
           "ftyp",
           8);
    memcpy(p + 8, movie ? "qt  " : "heic", 4);
    return bytes;
}
@implementation PHAsset
+ (PHFetchResult *)fetchAssetsWithLocalIdentifiers:(NSArray *)ids options:(id)options {
    assert(NO);
    return nil;
}
@end
@interface PHAssetResource ()
@property (nonatomic) BOOL unreadable;
@end
static PHAssetResource *FixtureResource(PHAssetResourceType type, NSString *name,
                                        NSString *uti) {
    PHAssetResource *resource = [PHAssetResource new];
    resource.type = type;
    resource.originalFilename = name;
    resource.uniformTypeIdentifier = uti;
    return resource;
}
@implementation PHAssetResource
+ (NSArray *)assetResourcesForAsset:(PHAsset *)asset {
    if ([asset.localIdentifier isEqual:@"raw-primary"])
        return @[
            FixtureResource(PHAssetResourceTypePhoto, @"primary.DNG", @"com.adobe.raw-image"),
            FixtureResource(PHAssetResourceTypeAlternatePhoto, @"alternate.JPG", @"public.jpeg")
        ];
    if ([asset.localIdentifier isEqual:@"jpeg-primary"])
        return @[
            FixtureResource(PHAssetResourceTypePhoto, @"primary.JPG", @"public.jpeg"),
            FixtureResource(PHAssetResourceTypeAlternatePhoto, @"alternate.DNG", @"com.adobe.raw-image")
        ];
    if ([asset.localIdentifier isEqual:@"raw-only"])
        return @[ FixtureResource(PHAssetResourceTypePhoto, @"primary.DNG", @"com.adobe.raw-image") ];
    if ([asset.localIdentifier isEqual:@"heic-primary"])
        return @[
            FixtureResource(PHAssetResourceTypePhoto, @"primary.HEIC", @"public.heic"),
            FixtureResource(PHAssetResourceTypeAlternatePhoto, @"alternate.JPG", @"public.jpeg")
        ];
    if ([asset.localIdentifier isEqual:@"raw-extension"])
        return @[
            FixtureResource(PHAssetResourceTypePhoto, @"primary.NEF", @"public.data"),
            FixtureResource(PHAssetResourceTypeAlternatePhoto, @"alternate.jpeg", @"public.data")
        ];
    if ([asset.localIdentifier isEqual:@"raw-uti"])
        return @[
            FixtureResource(PHAssetResourceTypePhoto, @"primary.bin", @"com.adobe.raw-image"),
            FixtureResource(PHAssetResourceTypeAlternatePhoto, @"alternate.bin", @"public.jpeg")
        ];
    PHAssetResource *photo = [PHAssetResource new];
    photo.type = PHAssetResourceTypePhoto;
    photo.originalFilename =
        asset.localIdentifier.intValue % 2 ? @"original.HEIC" : @"original.heif";
    photo.unreadable = IncludeUnreadable && asset.localIdentifier.intValue == 30;
    if (asset.mediaSubtypes & PHAssetMediaSubtypePhotoLive) {
        PHAssetResource *video = [PHAssetResource new];
        video.type = PHAssetResourceTypePairedVideo;
        video.originalFilename = @"original.MOV";
        return @[ photo, video ];
    }
    return @[ photo ];
}
@end
@implementation PHAssetResourceRequestOptions
@end
@implementation PHAssetResourceManager
+ (instancetype)defaultManager {
    static id manager;
    static dispatch_once_t once;
    dispatch_once(&once, ^{
        manager = [self new];
    });
    return manager;
}
- (void)writeDataForAssetResource:(PHAssetResource *)resource
                           toFile:(NSURL *)url
                          options:(PHAssetResourceRequestOptions *)options
                completionHandler:(void (^)(NSError *))completion {
    assert(!NSThread.isMainThread && options.networkAccessAllowed);
    int active = atomic_fetch_add(&ActiveExports, 1) + 1;
    if (active > atomic_load(&PeakExports))
        atomic_store(&PeakExports, active);
    assert(active == 1);
    [NSThread sleepForTimeInterval:0.001];
    atomic_fetch_sub(&ActiveExports, 1);
    if (resource.unreadable) {
        completion([NSError errorWithDomain:@"private-resource-error" code:99 userInfo:nil]);
        return;
    }
    assert([url.lastPathComponent isEqual:resource.originalFilename]);
    Written++;
    NSError *error = nil;
    [OriginalBytes(resource.type == PHAssetResourceTypePairedVideo) writeToURL:url
                                                                       options:0
                                                                         error:&error];
    completion(error);
}
@end
BOOL GSNativeIdentityMatches(NSString *identifier) {
    assert(NSThread.isMainThread);
    return [identifier isEqual:@"fixture"];
}
NSDictionary *GSRequest(NSDictionary *request, NSError **error) {
    assert(!NSThread.isMainThread);
    NSString *op = request[@"op"];
    if ([op isEqual:@"accounts"])
        return @{@"selected" : @"fixture@example.com"};
    if ([op isEqual:@"options"])
        return @{@"quality" : @"original"};
    if ([op isEqual:@"begin"]) {
        assert([request[@"quality"] isEqual:@"original"] &&
               [request[@"account"] isEqual:@"fixture@example.com"] &&
               [request[@"timestamp"] longLongValue] == 123);
        ExpectedResources = request[@"resources"];
        Received = [NSMutableArray array];
        for (NSDictionary *resource in ExpectedResources) {
            assert([resource[@"size"] intValue] == 70013);
            [Received addObject:[NSMutableData data]];
        }
        return @{@"id" : @"fixture-job"};
    }
    if ([op isEqual:@"append"]) {
        NSMutableData *bytes = Received [[request [@"index"] unsignedIntegerValue]];
        assert(bytes.length == [request[@"offset"] unsignedIntegerValue]);
        NSData *chunk = [[NSData alloc] initWithBase64EncodedString:request[@"data"] options:0];
        assert(chunk.length > 0 && chunk.length <= 32768);
        [bytes appendData:chunk];
        return @{};
    }
    if ([op isEqual:@"seal"]) {
        for (NSUInteger i = 0; i < Received.count; i++) {
            BOOL movie = [ExpectedResources[i][@"name"] isEqual:@"original.MOV"];
            assert([Received[i] isEqual:OriginalBytes(movie)]);
        }
        Queued++;
        return @{@"id" : @"fixture-job"};
    }
    assert(NO);
    return nil;
}
static NSDictionary *Run(void) {
    __block NSDictionary *done = nil;
    assert(GSStartBatchImport(
        60, @"album", YES,
        ^id(NSUInteger index) {
            PHAsset *asset = [PHAsset new];
            asset.localIdentifier = [NSString stringWithFormat:@"%lu", (unsigned long)index];
            asset.creationDate = [NSDate dateWithTimeIntervalSince1970:123];
            asset.mediaType = PHAssetMediaTypeImage;
            if (index == 5)
                asset.mediaSubtypes = PHAssetMediaSubtypePhotoLive;
            return asset;
        },
        @"fixture@example.com", @"fixture", nil,
        ^(NSDictionary *state) {
            done = state;
        }));
    NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:20];
    while (!done && deadline.timeIntervalSinceNow > 0)
        [NSRunLoop.currentRunLoop runUntilDate:[NSDate dateWithTimeIntervalSinceNow:0.001]];
    assert(done);
    return done;
}

static void CheckRAWJPEGSelection(void) {
    // Both entry points (native and bulk import) call this same exporter.
    NSArray<NSDictionary *> *cases = @[
        @{@"id" : @"raw-primary", @"expected" : @"alternate.JPG"},
        @{@"id" : @"jpeg-primary", @"expected" : @"primary.JPG"},
        @{@"id" : @"raw-only", @"expected" : @"primary.DNG"},
        @{@"id" : @"heic-primary", @"expected" : @"primary.HEIC"},
        @{@"id" : @"raw-extension", @"expected" : @"alternate.jpeg"},
        @{@"id" : @"raw-uti", @"expected" : @"alternate.bin"}
    ];
    dispatch_sync(dispatch_get_global_queue(QOS_CLASS_UTILITY, 0), ^{
        for (NSDictionary *fixture in cases) {
            @autoreleasepool {
                PHAsset *asset = [PHAsset new];
                asset.localIdentifier = fixture[@"id"];
                asset.mediaType = PHAssetMediaTypeImage;
                NSURL *directory = [NSURL
                    fileURLWithPath:[NSTemporaryDirectory()
                                        stringByAppendingPathComponent:NSUUID.UUID.UUIDString]
                        isDirectory:YES];
                NSError *error = nil;
                assert([NSFileManager.defaultManager createDirectoryAtURL:directory
                                              withIntermediateDirectories:YES
                                                               attributes:nil
                                                                    error:&error]);
                NSArray<NSURL *> *files = GSExportAsset(asset, directory, &error);
                assert(!error && files.count == 1);
                assert([files[0].lastPathComponent isEqual:fixture[@"expected"]]);
                assert([[NSData dataWithContentsOfURL:files[0]] isEqual:OriginalBytes(NO)]);
                [NSFileManager.defaultManager removeItemAtURL:directory error:nil];
            }
        }
    });
}
int main(void) {
    @autoreleasepool {
        CheckRAWJPEGSelection();
        NSDictionary *result = Run();
        assert(Queued == 60 && Written == 61 && [result[@"queued"] intValue] == 60 &&
               [result[@"failed"] intValue] == 0);
        IncludeUnreadable = YES;
        result = Run();
        assert(Queued == 119 && [result[@"queued"] intValue] == 59 &&
               [result[@"failed"] intValue] == 1 && [result[@"remaining"] intValue] == 0);
        assert([result[@"failureCodes"][@"export_failed"] intValue] == 1);
        IncludeUnreadable = NO;
        dispatch_group_t group = dispatch_group_create();
        for (NSUInteger i = 0; i < 60; i++)
            dispatch_group_async(group, dispatch_get_global_queue(QOS_CLASS_UTILITY, 0), ^{
                @autoreleasepool {
                    PHAsset *asset = [PHAsset new];
                    asset.mediaType = PHAssetMediaTypeImage;
                    asset.localIdentifier = @"native";
                    NSURL *directory = [NSURL
                        fileURLWithPath:[NSTemporaryDirectory()
                                            stringByAppendingPathComponent:NSUUID.UUID.UUIDString]
                            isDirectory:YES];
                    assert([NSFileManager.defaultManager createDirectoryAtURL:directory
                                                  withIntermediateDirectories:YES
                                                                   attributes:nil
                                                                        error:nil]);
                    NSArray *files = GSExportAsset(asset, directory, nil);
                    assert(files.count == 1);
                    assert([[NSData dataWithContentsOfURL:files[0]] isEqual:OriginalBytes(NO)]);
                    [NSFileManager.defaultManager removeItemAtURL:directory error:nil];
                }
            });
        assert(dispatch_group_wait(group, dispatch_time(DISPATCH_TIME_NOW, 10 * NSEC_PER_SEC)) ==
               0);
        assert(atomic_load(&PeakExports) == 1);
        NSLog(@"PASS 60 HEIC/HEIF originals, Live Photo resources, exact IPC bytes/timestamp, "
              @"unreadable original isolation, RAW+JPEG preference and bounded native exports");
    }
}
