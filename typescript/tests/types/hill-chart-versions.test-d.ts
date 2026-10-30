/** Public imports must name the same types accepted by listing and posting a hill chart's progress updates. */
import type {
  BasecampClient,
  CreateVersionHillChartRequest,
  ListVersionsHillChartOptions,
} from "../../src/index.js";

type Expect<T extends true> = T;
type Same<A, B> = [A] extends [B] ? ([B] extends [A] ? true : false) : false;
type ListOptions = NonNullable<Parameters<BasecampClient["hillCharts"]["listVersions"]>[1]>;
type CreateRequest = Parameters<BasecampClient["hillCharts"]["createVersion"]>[1];

export type HillChartVersionsAssertions = [
  Expect<Same<ListVersionsHillChartOptions, ListOptions>>,
  Expect<Same<ListVersionsHillChartOptions["page"], number | undefined>>,
  Expect<Same<CreateVersionHillChartRequest, CreateRequest>>,
  Expect<Same<NonNullable<CreateVersionHillChartRequest["dots"]>[number], { id: number; position: number }>>,
];
