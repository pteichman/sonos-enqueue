sonos-enqueue
=============

The Sonos tools don't provide any way to enqueue URLs to play,
preferring to support their partner APIs instead. This makes plenty of
sense but sometimes you just want to play a bunch of URLs.

This sonos-enqueue tool searches for local Sonos devices and adds any
URLs it has been passed to the specified device.

By default it replaces the device's queue entirely. Pass the `-a`
option to append instead.

To use:

    $ go install github.com/pteichman/sonos-enqueue@latest
    $ sonos-enqueue -d "Living Room" https://archive.org/download/gd77-05-08.sbd.hicks.4982.sbeok.shnf/gd77-05-08.sbd.hicks.4982.sbeok.shnf_vbr.m3u
