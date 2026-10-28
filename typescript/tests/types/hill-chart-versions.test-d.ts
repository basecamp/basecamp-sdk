/** Public imports must name the same options accepted by listing a hill chart's progress updates. */
import type { BasecampClient, ListVersionsHillChartOptions } from "../../src/index.js";

type Expect<T extends true> = T;
type Same<A, B> = [A] extends [B] ? ([B] extends [A] ? true : false) : false;
type ListOptions = NonNullable<Parameters<BasecampClient["hillCharts"]["listVersions"]>[1]>;

export type HillChartVersionsAssertions = [
  Expect<Same<ListVersionsHillChartOptions, ListOptions>>,
  Expect<Same<ListVersionsHillChartOptions["page"], number | undefined>>,
];
