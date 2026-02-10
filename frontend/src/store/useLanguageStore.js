import { create } from "zustand";
import { persist, createJSONStorage } from "zustand/middleware";
import { immer } from "zustand/middleware/immer";
import zh from "../locales/zh.json";
import fa from "../locales/fa.json";
import en from "../locales/en.json";

const translations = {
    zh,
    fa,
    en
};

export const useLanguageStore = create(
    persist(
        immer((set, get) => ({
            language: 'zh', // default to Chinese
            direction: 'ltr',

            setLanguage: (lang) =>
                set((state) => {
                    state.language = lang;
                    state.direction = lang === 'fa' ? 'rtl' : 'ltr';

                    // Update document direction
                    document.documentElement.lang = lang;
                    document.documentElement.dir = state.direction;
                }),

            t: (key, params = {}) => {
                const lang = get().language;
                let text = translations[lang]?.[key] || translations['zh']?.[key] || key;

                // Simple interpolation for {param}
                Object.keys(params).forEach(param => {
                    text = text.replace(new RegExp(`{${param}}`, 'g'), params[param]);
                });

                return text;
            }
        })),
        {
            name: 'language-storage',
            storage: createJSONStorage(() => localStorage),
            onRehydrateStorage: () => (state) => {
                if (state) {
                    document.documentElement.lang = state.language;
                    document.documentElement.dir = state.direction;
                }
            },
        }
    )
);
