# dclextract

`dclextract` is a command-line utility written in Go for extracting files from various proprietary archive formats.

## Features

-   **Automatic Format Detection:** Automatically detects the archive type by inspecting file headers and footers.
-   **Support for Multiple Formats:** Can extract files from `CMZ`, `NSK`, `TSC`, and `ZAR` archives.
-   **Robust Extraction:** In case of an error, the tool will attempt to write any files that were successfully extracted before the error occurred.
-   **Handles Nameless Files:** Generates sensible filenames (e.g., `archive_name_0`) for files that are stored without a name in the archive.

## Supported Formats

-   `CMZ` - [Ami Pro compressed distribution format](http://fileformats.archiveteam.org/wiki/CMZ_(archive_format))
-   `NSK` - [NaShrink](http://fileformats.archiveteam.org/wiki/NaShrinK)
-   `TSC` - [The Stirling Group Compresssor](http://fileformats.archiveteam.org/wiki/TSComp)
-   `ZAR` - [Zip-Archiv](http://fileformats.archiveteam.org/wiki/ZAR_(Zip-Archiv))

## Installation

To build the `dclextract` tool from source, you need to have a working Go environment.

1.  Clone the repository:
    ```sh
    git clone https://github.com/sourcekris/dclextract.git
    cd dclextract
    ```

2.  Build the executable:
    ```sh
    go build
    ```
    This will create a `dclextract` (or `dclextract.exe` on Windows) executable in the current directory.

Alternatively, you can install it directly into your `$GOPATH/bin` using `go install`:
```sh
go install github.com/sourcekris/dclextract@latest
```

## Usage

To extract files from an archive, provide the path to the archive file as a command-line argument. The extracted files will be saved in the current working directory.

```sh
./dclextract <path/to/archive.ext>
```

### Example

**Extracting a typical archive:**

```sh
$ ./dclextract my_data.zar
Detected file type: ZAR
Successfully extracted file1.txt (compressed: 1024 bytes, uncompressed: 2048 bytes) to file1.txt
Successfully extracted image.bmp (compressed: 51200 bytes, uncompressed: 153600 bytes) to image.bmp
```

**Extracting an archive with nameless files:**

If a file inside the archive has no name, `dclextract` will generate one based on the archive's name.

```sh
$ ./dclextract assets.cmz
Detected file type: CMZ
No filename found in archive for item 1, using generated name: assets_0
Successfully extracted  (compressed: 400 bytes, uncompressed: 1200 bytes) to assets_0
```