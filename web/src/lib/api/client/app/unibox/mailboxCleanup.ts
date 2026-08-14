import Request from "../../Request";
import type {
  MailboxCleanupPreview,
  MailboxCleanupResult,
} from "@/lib/api/models/app/unibox/MailboxCleanup";

interface DataResponse<T> {
  data: T;
}

export async function previewMailboxCleanup(
  emailAccountIds: string[],
): Promise<MailboxCleanupPreview> {
  const response = await Request<DataResponse<MailboxCleanupPreview>>({
    method: "POST",
    url: "/unibox/mailbox-cleanup/preview",
    data: { email_account_ids: emailAccountIds },
    authorization: true,
  });
  return response.data;
}

export async function runMailboxCleanup(
  emailAccountIds: string[],
): Promise<MailboxCleanupResult> {
  const response = await Request<DataResponse<MailboxCleanupResult>>({
    method: "POST",
    url: "/unibox/mailbox-cleanup",
    data: { email_account_ids: emailAccountIds, confirm: true },
    authorization: true,
  });
  return response.data;
}
