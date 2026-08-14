import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  previewMailboxCleanup,
  runMailboxCleanup,
} from "@/lib/api/client/app/unibox/mailboxCleanup";

export default function useMailboxCleanup() {
  const queryClient = useQueryClient();
  const preview = useMutation({ mutationFn: previewMailboxCleanup });
  const cleanup = useMutation({
    mutationFn: runMailboxCleanup,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["unibox", "overview"] });
      queryClient.invalidateQueries({ queryKey: ["unibox", "search"] });
      queryClient.invalidateQueries({ queryKey: ["unibox"] });
    },
  });
  return { preview, cleanup };
}
