# Agent session files

The browser transfers files through Manager and the authenticated Edge RPC channel.
Codex does not transport file bytes. Each chunk reauthorizes the user, saved access,
connector ownership and file feature permissions. Edge independently verifies its
durable user/access/session binding and saved project.

## Boundaries and limits

- Content access is restricted to the session's working directory using `os.Root`.
  Directory discovery's broader home-directory scope does not grant file access.
- Uploads create unique files in `.liaison-attachments/<session-id>/`, mode 0600.
  They never replace existing project files. Removing a composer attachment only
  detaches it from the pending message; it does not delete the uploaded file.
- Each file is limited to 20 MiB; each message to eight attachments. Native image
  inputs support PNG, JPEG and GIF, up to 4 MiB and 40 million pixels per image.
- Transfers use 128 KiB chunks, with exact offsets and at most eight live transfers
  per Edge. Abandoned transfers expire after two minutes. Partial uploads are hidden
  and cannot be downloaded; commit publishes the complete file without overwrite.
- Downloads hold an open file descriptor and reject files changed during transfer.
  They are saved as attachments, never rendered as HTML or executed.
- Image content is validated and copied into an Edge-owned temporary snapshot before
  passing `localImage` to Codex. These exact snapshots are removed on the next turn
  or session close. Original uploads remain in the project.
- Manager persists attachment names, paths and sizes with encrypted conversation
  history. It does not persist transfer chunks or image bytes in SQLite.

## UI and compatibility

The composer supports file selection, image selection, file drop and image paste.
Uploads have progress/cancel, local image previews and removable attachment chips.
Project files can be browsed from the session header; clicking a file opens its
preview and a separate action downloads it. Sent attachment
chips also provide download actions. File content requires an online connector.

Conversation file links open a resizable right-side preview (full-screen on small
screens), with syntax highlighting, line numbers, line-range navigation, copying
and download. HTTP(S) links remain external links. Relative paths and absolute paths
inside the current project are converted to project-relative reads; invalid or
out-of-project paths do not trigger a read. Edge still enforces the actual boundary,
including symlink traversal, for every request. Directory links use the same scoped
file browser API. Previewing does not execute Codex commands or synchronize a repo.

Previews show current file contents, not historical revisions. UTF-8 text previews
are limited to 1 MiB and 10,000 lines; syntax highlighting is limited to 200,000
characters and is loaded on demand. Other files can be downloaded within the
existing transfer limit. Highlighting produces React text/span nodes, not raw HTML.
Closing a preview aborts reads; unfinished known transfers are cancelled. Preview
contents remain in browser memory, not conversation history or browser storage.

README (without an extension), `.md` and `.markdown` files default to the shared
safe Markdown preview, with a source toggle. Links with line targets default to
source so line navigation remains precise. Markdown rendering is capped at 200,000
characters; larger documents fall back to source. Raw HTML and embedded images are
not loaded. Project-relative document links, source line links and heading anchors
open inside the preview with back navigation. Parent segments are normalized only
within the project; paths escaping the project are rejected before a file read.
Heading navigation reuses the loaded contents and only scrolls the preview pane.
HTTP(S) links can be opened explicitly in a separate tab.

The optional `files_available` / `files_upload_available` capabilities gate controls.
Both Manager and Edge must be upgraded; older connectors remain usable for chat
without file controls. Native model/configuration selection is unchanged. The current
Agent runtime supports macOS and Linux; this does not add Windows Agent support.
