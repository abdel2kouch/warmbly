import deleteCampaign from "@/lib/api/client/app/campaigns/deleteCampaign";
import type GetCampaigns from "@/lib/api/models/app/campaigns/GetCampaigns";
import { type InfiniteData, useMutation, useQueryClient } from "@tanstack/react-query";

export default function useDeleteCampaign() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: async (id: string) => deleteCampaign(id),
        onSuccess: (_data, variables) => {
            const allLists = queryClient.getQueriesData<InfiniteData<GetCampaigns>>({
                queryKey: ["campaigns", "list"],
            });

            for (const [key, oldData] of allLists) {
                if (!oldData) continue;

                queryClient.setQueryData(key, {
                    ...oldData,
                    pages: oldData.pages.map((page) => ({
                        ...page,
                        data: page.data.filter((c) => c.id !== variables),
                    })),
                });
            }

            queryClient.invalidateQueries({
                queryKey: ["campaigns", variables]
            });
        }
    })
}
