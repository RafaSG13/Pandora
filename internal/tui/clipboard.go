package tui

// writeSecretClipboard puts a secret value on the system clipboard, marked
// (where the OS has a convention for it) so clipboard-history managers and
// cloud clipboard sync skip it: org.nspasteboard.ConcealedType on macOS, the
// history/monitor/cloud exclusion formats on Windows. Linux has no standard
// marker and gets a plain write. A variable so tests never touch the
// developer's real clipboard; platformWriteSecret lives in clipboard_<os>.go.
var writeSecretClipboard = platformWriteSecret
