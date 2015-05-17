# flac2one

The tool converts a bunch of FLAC files into one FLAC and a CUE-sheet file.

It also saves tags in CUE file and picture (cover image) in result flac file.

## Usage

### Converting
```
$ flac2one "Nine Inch Nails"/"[Halo 02R] 1989 - Pretty hate machine [2010, B0015099-02]"/*.flac
Processing: Nine Inch Nails/[Halo 02R] 1989 - Pretty hate machine [2010, B0015099-02]/01. Head Like a Hole.flac
Processing: Nine Inch Nails/[Halo 02R] 1989 - Pretty hate machine [2010, B0015099-02]/02. Terrible Lie.flac
Processing: Nine Inch Nails/[Halo 02R] 1989 - Pretty hate machine [2010, B0015099-02]/03. Down In It.flac
Processing: Nine Inch Nails/[Halo 02R] 1989 - Pretty hate machine [2010, B0015099-02]/04. Sanctified.flac
Processing: Nine Inch Nails/[Halo 02R] 1989 - Pretty hate machine [2010, B0015099-02]/05. Something I Can Never Have.flac
Processing: Nine Inch Nails/[Halo 02R] 1989 - Pretty hate machine [2010, B0015099-02]/06. Kinda I Want To.flac
Processing: Nine Inch Nails/[Halo 02R] 1989 - Pretty hate machine [2010, B0015099-02]/07. Sin.flac
Processing: Nine Inch Nails/[Halo 02R] 1989 - Pretty hate machine [2010, B0015099-02]/08. That's What I Get.flac
Processing: Nine Inch Nails/[Halo 02R] 1989 - Pretty hate machine [2010, B0015099-02]/09. The Only Time.flac
Processing: Nine Inch Nails/[Halo 02R] 1989 - Pretty hate machine [2010, B0015099-02]/10. Ringfinger.flac
Processing: Nine Inch Nails/[Halo 02R] 1989 - Pretty hate machine [2010, B0015099-02]/11. Get Down Make Love.flac
Writing to "./Nine Inch Nails - Pretty hate machine [2010, UMe, B0015099-02].[flac|cue]"
```

### Options
```
    -s, --silent        Silent mode
    -d, --delete        Delete input files after processing
    -o, --output=DIR    Output directory (defaults to current dir)
```

## Behaviour (Known bugs)

* Command line arguments sets the order of the tracks
* Tool takes tags ARTIST, DATE and GENRE only from first file and saves it to CUE-file
* Title for each track is generated from tag TITLE
* Picture is taken only from first file and only if its type is "Cover (front)"
* Seektable is recalculated, points are set every 10 seconds
* Result flac file is always variable block-size type

## Requirements

* golang

## Installation

```
$ go install github.com/sdidyk/flac2one
```

## License

MIT

## Author

Sergey Didyk
