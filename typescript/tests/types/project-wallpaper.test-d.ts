/** Public imports must name the same options accepted by the wallpaper upload. */
import type { BasecampClient, UpdateProjectWallpaperProjectOptions } from "../../src/index.js";

type Expect<T extends true> = T;
type Same<A, B> = [A] extends [B] ? ([B] extends [A] ? true : false) : false;
type UploadOptions = NonNullable<Parameters<BasecampClient["projects"]["updateProjectWallpaper"]>[3]>;

export type ProjectWallpaperAssertions = [
  Expect<Same<UpdateProjectWallpaperProjectOptions, UploadOptions>>,
  Expect<Same<UpdateProjectWallpaperProjectOptions["mode"], string | undefined>>,
];
