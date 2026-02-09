import { create } from "zustand";
import { persist, createJSONStorage } from "zustand/middleware";
import { immer } from "zustand/middleware/immer";

export const useStartStore = create(
  immer((set) => ({
    type: 'newbie',
    step: 'letsencrypt',
    templateType: undefined,
    setType: (type) =>
      set((state) => {
        state.type = type
      }),
    setStep: (step) =>
      set((state) => {
        state.step = step
      }),
    setTemplateType: (type) =>
      set((state) => {
        state.templateType = type
      })
  })),
);