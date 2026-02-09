import { useGet, useTrigger } from "../utils/fetcher";

export function useGetTemplates() {
    return useGet({
        url: '/templates'
    });
}

export function useInitTemplate() {
    return useTrigger({
        url: `/templates/init`,
        method: 'POST'
    })
}

export function useCreateTemplate() {
    return useTrigger({
        url: `/templates/create`,
        method: 'POST'
    })
}