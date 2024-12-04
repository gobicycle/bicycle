# TypeScript Template

This is a TypeScript project template using webpack for Spck for NodeJS. This project template was made with the help of `createapp.dev`.

## Building and Running

Pressing the ▶ button will call the command `build` in `package.json`. If `dist/bundle.js` file does not exist, it indicates this may be the first run and `install-dep` in `package.json` will be called. The `spck.config.json` file controls which command to call when pressing ▶ (which can be modified in **Run Settings**).

The task `build` creates a development build of the project and generates:

- `dist/bundle.js`

When `build` finishes, the preview window will launch.

## Limitations in Android

Due to security restrictions in Android, execute permissions on write-allowed storage is likely forbidden on most stock devices. This prevents some npm scripts from working properly as `npm run` rely on the use of `sh` which requires exec permissions.

The `node` program is also built as a shared library for compatibility with future versions of Android and can only be accessed from the terminal and not `sh`.

For these reasons, using `npm run ...` will not work from the terminal, but entering the command (`webpack`) directly in the terminal will work.

## NPM Install

On external storage and SD cards, it is commonly using FAT32 or exFAT filesystems. These filesystems do not support symbolic links which is why npm dependencies that uses symlinks (mostly npm dependencies with command line usage symlinks) will fail on external storage.

Add the `--no-bin-links` option to `npm install` to prevent creation of symlinks.

```bash
npm i @babel/preset-env --no-bin-links
```
