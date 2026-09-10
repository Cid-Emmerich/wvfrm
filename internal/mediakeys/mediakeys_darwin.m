//go:build darwin

#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>
#import <MediaPlayer/MediaPlayer.h>
#include "_cgo_export.h"

// Commands mirror the Go Command constants.
enum { CMD_PLAY, CMD_PAUSE, CMD_TOGGLE, CMD_NEXT, CMD_PREV, CMD_STOP };

static MPRemoteCommandHandlerStatus fire(int cmd) {
    wvfrmMediaCommand(cmd);
    return MPRemoteCommandHandlerStatusSuccess;
}

void wvfrmMediaStart(void) {
    @autoreleasepool {
        // A bare command-line process has no NSApplication; creating the
        // shared application is enough for MediaPlayer to register us as a
        // Now Playing client without showing anything in the Dock.
        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyProhibited];

        MPRemoteCommandCenter *cc = [MPRemoteCommandCenter sharedCommandCenter];
        [cc.playCommand addTargetWithHandler:^(MPRemoteCommandEvent *e) { return fire(CMD_PLAY); }];
        [cc.pauseCommand addTargetWithHandler:^(MPRemoteCommandEvent *e) { return fire(CMD_PAUSE); }];
        [cc.togglePlayPauseCommand addTargetWithHandler:^(MPRemoteCommandEvent *e) { return fire(CMD_TOGGLE); }];
        [cc.nextTrackCommand addTargetWithHandler:^(MPRemoteCommandEvent *e) { return fire(CMD_NEXT); }];
        [cc.previousTrackCommand addTargetWithHandler:^(MPRemoteCommandEvent *e) { return fire(CMD_PREV); }];
        [cc.stopCommand addTargetWithHandler:^(MPRemoteCommandEvent *e) { return fire(CMD_STOP); }];
        cc.playCommand.enabled = YES;
        cc.pauseCommand.enabled = YES;
        cc.togglePlayPauseCommand.enabled = YES;
        cc.nextTrackCommand.enabled = YES;
        cc.previousTrackCommand.enabled = YES;
    }
}

void wvfrmMediaUpdate(const char *title, const char *artist, const char *album,
                      double duration, double position, int playing,
                      const void *art, int artLen) {
    @autoreleasepool {
        NSMutableDictionary *info = [NSMutableDictionary dictionary];
        info[MPMediaItemPropertyTitle] = [NSString stringWithUTF8String:title];
        info[MPMediaItemPropertyArtist] = [NSString stringWithUTF8String:artist];
        info[MPMediaItemPropertyAlbumTitle] = [NSString stringWithUTF8String:album];
        info[MPMediaItemPropertyPlaybackDuration] = @(duration);
        info[MPNowPlayingInfoPropertyElapsedPlaybackTime] = @(position);
        info[MPNowPlayingInfoPropertyPlaybackRate] = @(playing ? 1.0 : 0.0);
        info[MPNowPlayingInfoPropertyMediaType] = @(MPNowPlayingInfoMediaTypeAudio);
        if (art != NULL && artLen > 0) {
            NSData *data = [NSData dataWithBytes:art length:artLen];
            NSImage *img = [[NSImage alloc] initWithData:data];
            if (img != nil) {
                info[MPMediaItemPropertyArtwork] = [[MPMediaItemArtwork alloc]
                    initWithBoundsSize:img.size
                    requestHandler:^NSImage *(CGSize size) { return img; }];
            }
        }
        MPNowPlayingInfoCenter *c = [MPNowPlayingInfoCenter defaultCenter];
        c.nowPlayingInfo = info;
        c.playbackState = playing ? MPNowPlayingPlaybackStatePlaying : MPNowPlayingPlaybackStatePaused;
    }
}

void wvfrmMediaClear(void) {
    @autoreleasepool {
        MPNowPlayingInfoCenter *c = [MPNowPlayingInfoCenter defaultCenter];
        c.nowPlayingInfo = nil;
        c.playbackState = MPNowPlayingPlaybackStateStopped;
    }
}

void wvfrmMediaRunLoop(void) {
    CFRunLoopRun();
}

void wvfrmMediaStopLoop(void) {
    CFRunLoopRef main = CFRunLoopGetMain();
    CFRunLoopPerformBlock(main, kCFRunLoopCommonModes, ^{ CFRunLoopStop(main); });
    CFRunLoopWakeUp(main);
}
