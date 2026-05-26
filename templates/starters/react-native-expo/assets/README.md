# Assets

This starter ships without binary image assets. Before the first build, drop the
following PNG files into this directory. Filenames are referenced by `app.json`
and must match exactly.

| File                  | Size (px)   | Used for                                   |
| --------------------- | ----------- | ------------------------------------------ |
| `icon.png`            | 1024 x 1024 | App icon (iOS + Android fallback)          |
| `adaptive-icon.png`   | 1024 x 1024 | Android adaptive icon foreground           |
| `splash.png`          | 1284 x 2778 | Launch screen (portrait, 3x)               |
| `favicon.png`         | 48 x 48     | Web favicon (`web.favicon` in `app.json`)  |

## Background colors

The Android adaptive icon and the splash screen both use background
`#050507` (set in `app.json`). Use a transparent PNG for `adaptive-icon.png`
and let Expo composite the background.

## Generating placeholders

To unblock the first prebuild, you can generate flat-color placeholders:

```bash
# Requires ImageMagick
magick -size 1024x1024 xc:'#050507' icon.png
magick -size 1024x1024 xc:none adaptive-icon.png
magick -size 1284x2778 xc:'#050507' splash.png
magick -size 48x48 xc:'#050507' favicon.png
```

The Ironflyer asset gate will reject these as soon as you try to ship to a
store, but they are fine for local development.
