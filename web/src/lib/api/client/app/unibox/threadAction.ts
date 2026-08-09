import Request from "../../Request";

export type UniboxThreadAction = "archive" | "trash" | "mark_read" | "mark_unread";

export default async function threadAction(
  threadId: string,
  action: UniboxThreadAction,
): Promise<void> {
  return await Request<void>({
    method: "POST",
    url: "/unibox/thread/action",
    data: { thread_id: threadId, action },
    authorization: true,
  });
}
