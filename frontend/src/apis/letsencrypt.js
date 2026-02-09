import { useTrigger } from "../utils/fetcher";

export function useLetsEncryptInit() {
    return useTrigger({
        url: '/letsencrypt/init',
        method: 'POST'
    });
}

