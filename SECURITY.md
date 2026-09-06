# Security Policy

zhuri is an open source project maintained in spare time. There's no formal
SLA, but security reports are taken seriously and get a best-effort response
as fast as reasonably possible.

## Reporting a vulnerability

Please use GitHub's private vulnerability reporting: go to the **Security**
tab of this repository, then **Report a vulnerability**.

Do **not** open a public issue for a security problem. Reporting privately
means the details aren't sitting in public view — searchable, indexable,
exploitable — before a fix exists.

You should get an acknowledgement within a few days. If you don't hear back,
follow up on the same report thread rather than opening a new one.

## Scope

zhuri reads and rewrites files that belong to whoever runs it, so the bug
classes that actually matter here are the ones around file handling, not,
say, web-style injection. Specifically, these are the vectors worth a
private report:

- **Symlink traversal** — zhuri walking outside the directory tree it was
  pointed at, by following a symlink, when run with `-r`.
- **Special files** — zhuri reading from or writing to a device, socket, or
  named pipe instead of a regular file.
- **File corruption on write** — any path where `--write` can leave a file
  partially written, truncated, or otherwise damaged. Writes are meant to be
  atomic (temp file + rename); a bug that defeats that guarantee is a real
  security bug, not just a correctness bug.
- **Binary misdetection** — a binary file getting misclassified as text and
  transcoded, silently destroying its contents.
- **Resource exhaustion** — a crafted or oversized file causing excessive
  memory or CPU use. The size limit and bounded reads exist specifically to
  cap this; a way around them is in scope.

Bugs in the encoding-detection heuristic itself (guessing Windows-1252
instead of ISO-8859-1, say) are not security issues on their own — those are
regular bugs, please file them as a normal issue.

## Supported versions

| Version      | Supported          |
| ------------ | ------------------- |
| latest minor | :white_check_mark:  |
| older        | :x:                 |

Only the latest minor release gets security fixes. There's no long-term
support branch at this stage of the project.
