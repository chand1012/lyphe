#!/bin/sh
set -eu

mkdir -p /runtime/usr/bin
for binary in /usr/bin/ffmpeg /usr/bin/ffprobe; do
    cp "$binary" /runtime/usr/bin/
    dependencies=$(ldd "$binary")
    if printf '%s\n' "$dependencies" | grep -q 'not found'; then
        printf '%s\n' "$dependencies" >&2
        exit 1
    fi
    printf '%s\n' "$dependencies" | awk '
        /=> \// { print $3 }
        /^[[:space:]]*\// { print $1 }
    ' | while IFS= read -r library; do
        # Resolve usrmerge directories but retain SONAME filenames from ldd.
        directory=$(readlink -f "$(dirname "$library")")
        mkdir -p "/runtime$directory"
        cp -L "$library" "/runtime$directory/$(basename "$library")"
    done
done
