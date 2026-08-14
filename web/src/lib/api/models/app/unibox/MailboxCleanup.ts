export interface MailboxCleanupPreview {
  selected_mailboxes: number;
  messages_to_trash: number;
  preserved_threads: number;
  preserved_messages: number;
  unsupported_mailboxes: number;
}

export interface MailboxCleanupResult {
  selected_mailboxes: number;
  queued_for_trash: number;
  preserved_threads: number;
  preserved_messages: number;
}
