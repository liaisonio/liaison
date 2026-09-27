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
Project files can be browsed/downloaded from the session header; sent attachment
chips also provide download actions. File content requires an online connector.

The optional `files_available` / `files_upload_available` capabilities gate controls.
Both Manager and Edge must be upgraded; older connectors remain usable for chat
without file controls. Native model/configuration selection is unchanged. The current
Agent runtime supports macOS and Linux; this does not add Windows Agent support.
