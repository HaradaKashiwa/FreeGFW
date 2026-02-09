import { useGet, useTrigger } from "../utils/fetcher";

export function useGetConfigs() {
    return useGet({
        url: '/configs'
    });
}

export function useReloadConfigs() {
    return useTrigger({
        url: '/configs/reload',
        method: 'POST'
    });
}

export function useResetConfigs() {
    return useTrigger({
        url: '/configs/reset',
        method: 'POST'
    });
}

export function useSetTitle() {
    return useTrigger({
        url: '/configs/title',
        method: 'POST'
    });
}

export function useUpdateConfigs() {
    return useTrigger({
        url: '/configs/update',
        method: 'PUT'
    });
}