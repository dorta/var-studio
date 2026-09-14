# VAR-Studio Native Demos

These applications run on the physical display connected to the EVK. The
VAR-Studio web interface only discovers, starts, monitors, and stops them.

Build them with the matching Variscite Yocto SDK:

```sh
cmake -S demos -B build-demos \
  -DCMAKE_TOOLCHAIN_FILE="$OE_CMAKE_TOOLCHAIN_FILE"
cmake --build build-demos
cmake --install build-demos --prefix /usr
```

The binaries are installed in `/opt/var-studio-demos/bin`. A demo is exposed
as ready only when its executable exists on the running image.
