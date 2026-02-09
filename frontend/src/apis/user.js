import { useTrigger } from "../utils/fetcher";
import { useGet } from "../utils/fetcher";

export function useAddUsers() {
    return useTrigger({
        url: '/users',
        method: 'POST'
    });
}

export function useGetUsers() {
    return useGet({
        url: '/users'
    })
}

export function useDeleteUser({ id }) {
    return useTrigger({
        url: `/users/${id}`,
        method: 'DELETE'
    })
}