#!/usr/bin/env python3
"""Splice /tmp/user-message.txt into a prompt YAML's messages array."""
import sys, yaml

prompt_file = sys.argv[1]
output_file = sys.argv[2]

with open(prompt_file) as f:
    lines = f.readlines()
with open('/tmp/user-message.txt') as f:
    user_msg = f.read()

# Find where messages array ends: first non-blank top-level key after 'messages:'
insert_at = len(lines)
for i, line in enumerate(lines):
    if i == 0:
        continue
    if line.strip() and not line[0].isspace():
        insert_at = i
        break

entry = ['  - role: user\n', '    content: |\n']
for ln in user_msg.splitlines():
    entry.append('      ' + ln + '\n')

lines[insert_at:insert_at] = entry
with open(output_file, 'w') as f:
    f.writelines(lines)

# Validate: last message must be the user message we just added
doc = yaml.safe_load(open(output_file))
assert doc['messages'][-1]['role'] == 'user', 'prompt splice failed: last message is not user'
