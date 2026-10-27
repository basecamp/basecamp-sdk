import type { BasecampClient, Project } from "../../src/index.js";

type Expect<T extends true> = T;
type Same<A, B> = [A] extends [B] ? ([B] extends [A] ? true : false) : false;

export type ProjectLogoAssertions = [
  Expect<Same<Project["logo"], { url: string } | undefined>>,
  Expect<Same<ReturnType<BasecampClient["projects"]["updateProjectLogo"]>, Promise<void>>>,
  Expect<Same<ReturnType<BasecampClient["projects"]["removeProjectLogo"]>, Promise<void>>>,
];
