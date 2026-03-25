/**
 * Post-processes schema.d.ts to add system_label to Person type.
 *
 * The Basecamp API returns system actors (LocalPerson) through the regular
 * Person JSON shape with a non-numeric id (e.g. "basecamp"). The SDK
 * normalizes this at runtime: id becomes 0, and the original label is
 * preserved as system_label. This script adds the system_label field to
 * the generated Person type so consumers can access it with type safety.
 */
import { readFileSync, writeFileSync } from "fs";
import { resolve, dirname } from "path";
import { fileURLToPath } from "url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const schemaPath = resolve(__dirname, "../src/generated/schema.d.ts");

let content = readFileSync(schemaPath, "utf-8");

// Add system_label after Person.id
content = content.replace(
  /(\bPerson: \{\n\s+\/\*\* Format: int64 \*\/\n\s+id: number;)/,
  "$1\n            /** Label for system actors (e.g. \"basecamp\"). Present when personable_type is \"LocalPerson\". */\n            system_label?: string;"
);

if (content === readFileSync(schemaPath, "utf-8")) {
  console.error("ERROR: patch-flexible-ids.ts failed to match Person schema — schema.d.ts may have changed format");
  process.exit(1);
}
writeFileSync(schemaPath, content);
console.log("Patched Person type: added system_label in schema.d.ts");
