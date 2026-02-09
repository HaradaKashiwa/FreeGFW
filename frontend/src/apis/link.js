import { useTrigger, useGet } from "../utils/fetcher";

export function useGetLinks() {
    return useGet({
        url: '/link/list'
    })
}

export function useCreateLink() {
    return useTrigger({
        url: '/link/create',
        method: 'POST'
    })
}

export function useSwapLink() {
    return useTrigger({
        url: '/link/swap',
        method: 'POST'
    })
}

export function useDeleteLink({ id }) {
    return useTrigger({
        url: `/link/${id}`,
        method: 'DELETE'
    })
}

