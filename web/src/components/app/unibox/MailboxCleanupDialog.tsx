import React from "react";
import { Loader2Icon, ShieldCheckIcon, Trash2Icon } from "lucide-react";
import toast from "react-hot-toast";

import type { UniboxMailboxOverview } from "@/lib/api/models/app/unibox/UniboxOverview";
import type { MailboxCleanupPreview } from "@/lib/api/models/app/unibox/MailboxCleanup";
import useMailboxCleanup from "@/lib/api/hooks/app/unibox/useMailboxCleanup";
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";

interface MailboxCleanupDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  mailboxes: UniboxMailboxOverview[];
}

export function MailboxCleanupDialog({
  open,
  onOpenChange,
  mailboxes,
}: MailboxCleanupDialogProps) {
  const { preview: previewRequest, cleanup } = useMailboxCleanup();
  const [selected, setSelected] = React.useState<string[]>([]);
  const [preview, setPreview] = React.useState<MailboxCleanupPreview | null>(null);
  const [confirmed, setConfirmed] = React.useState(false);

  const reset = React.useCallback(() => {
    setSelected([]);
    setPreview(null);
    setConfirmed(false);
    previewRequest.reset();
    cleanup.reset();
  }, [cleanup, previewRequest]);

  const handleOpenChange = (next: boolean) => {
    if (!next) reset();
    onOpenChange(next);
  };

  const toggleMailbox = (id: string) => {
    setSelected((current) =>
      current.includes(id) ? current.filter((currentID) => currentID !== id) : [...current, id],
    );
    setPreview(null);
    setConfirmed(false);
  };

  const requestPreview = async () => {
    try {
      setPreview(await previewRequest.mutateAsync(selected));
      setConfirmed(false);
    } catch {
      toast.error("Couldn’t prepare the cleanup preview.");
    }
  };

  const runCleanup = async () => {
    try {
      const result = await cleanup.mutateAsync(selected);
      toast.success(
        `${result.queued_for_trash} ${result.queued_for_trash === 1 ? "message was" : "messages were"} queued for Gmail Trash.`,
      );
      handleOpenChange(false);
    } catch {
      toast.error("Couldn’t queue this mailbox cleanup.");
    }
  };

  const unsupported = (preview?.unsupported_mailboxes ?? 0) > 0;
  const executeDisabled = !preview || unsupported || !confirmed || cleanup.isPending;

  return (
    <AlertDialog open={open} onOpenChange={handleOpenChange}>
      <AlertDialogContent className="max-w-lg">
        <AlertDialogHeader>
          <AlertDialogTitle>Clean selected mailboxes</AlertDialogTitle>
          <AlertDialogDescription>
            Move selected Gmail messages to Trash while preserving every conversation where a lead contacted through a Warmbly campaign replied.
          </AlertDialogDescription>
        </AlertDialogHeader>

        <div className="space-y-2">
          <p className="text-xs font-semibold text-slate-700">Choose Gmail mailboxes</p>
          <div className="max-h-48 overflow-y-auto rounded-md border border-slate-200 divide-y divide-slate-100">
            {mailboxes.length === 0 ? (
              <p className="px-3 py-4 text-sm text-slate-500">No connected mailboxes are available.</p>
            ) : (
              mailboxes.map((mailbox) => {
                const checked = selected.includes(mailbox.id);
                return (
                  <label key={mailbox.id} className="flex cursor-pointer items-center gap-3 px-3 py-2.5 hover:bg-slate-50">
                    <input
                      type="checkbox"
                      checked={checked}
                      onChange={() => toggleMailbox(mailbox.id)}
                      className="size-4 rounded border-slate-300 text-sky-600 focus:ring-sky-500"
                    />
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-sm font-medium text-slate-800">{mailbox.name || mailbox.email}</span>
                      {mailbox.name && <span className="block truncate text-xs text-slate-500">{mailbox.email}</span>}
                    </span>
                    <span className="text-xs tabular-nums text-slate-400">{mailbox.total} messages</span>
                  </label>
                );
              })
            )}
          </div>
        </div>

        {preview && (
          <div className="space-y-2 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-950">
            <div className="flex gap-2">
              <ShieldCheckIcon className="mt-0.5 size-4 shrink-0 text-amber-700" />
              <div>
                <p><strong>{preview.messages_to_trash}</strong> messages will move to Gmail Trash.</p>
                <p><strong>{preview.preserved_messages}</strong> messages in <strong>{preview.preserved_threads}</strong> campaign-lead reply conversations will be preserved.</p>
              </div>
            </div>
            {unsupported ? (
              <p className="text-red-700">{preview.unsupported_mailboxes} selected mailbox{preview.unsupported_mailboxes === 1 ? " is" : "es are"} not active Gmail mailboxes. Remove them before continuing.</p>
            ) : (
              <label className="flex cursor-pointer items-start gap-2 pt-1 text-xs text-amber-900">
                <input
                  type="checkbox"
                  checked={confirmed}
                  onChange={(event) => setConfirmed(event.target.checked)}
                  className="mt-0.5 size-3.5 rounded border-amber-400 text-red-600 focus:ring-red-500"
                />
                <span>I understand this moves the listed messages to Gmail Trash. It does not permanently delete them.</span>
              </label>
            )}
          </div>
        )}

        <AlertDialogFooter>
          <AlertDialogCancel disabled={previewRequest.isPending || cleanup.isPending}>Cancel</AlertDialogCancel>
          {!preview ? (
            <button
              type="button"
              disabled={selected.length === 0 || previewRequest.isPending}
              onClick={requestPreview}
              className="inline-flex h-9 items-center justify-center gap-2 rounded-md bg-slate-900 px-3 text-sm font-medium text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {previewRequest.isPending && <Loader2Icon className="size-4 animate-spin" />}
              Preview cleanup
            </button>
          ) : (
            <button
              type="button"
              disabled={executeDisabled}
              onClick={runCleanup}
              className="inline-flex h-9 items-center justify-center gap-2 rounded-md bg-red-600 px-3 text-sm font-medium text-white hover:bg-red-700 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {cleanup.isPending ? <Loader2Icon className="size-4 animate-spin" /> : <Trash2Icon className="size-4" />}
              Move {preview.messages_to_trash} to Trash
            </button>
          )}
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
