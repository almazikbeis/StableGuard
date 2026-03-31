export type ToastLevel = "info" | "success" | "warning" | "danger";

export interface ToastItem {
  id: string;
  level: ToastLevel;
  title: string;
  message?: string;
}

type Listener = (items: ToastItem[]) => void;

let items: ToastItem[] = [];
const listeners = new Set<Listener>();
let counter = 0;

function emit() {
  listeners.forEach((l) => l([...items]));
}

export const toast = {
  show(level: ToastLevel, title: string, message?: string, durationMs = 4500) {
    const id = `t-${++counter}`;
    items = [...items, { id, level, title, message }];
    emit();
    setTimeout(() => {
      items = items.filter((t) => t.id !== id);
      emit();
    }, durationMs);
  },
  dismiss(id: string) {
    items = items.filter((t) => t.id !== id);
    emit();
  },
  subscribe(listener: Listener) {
    listeners.add(listener);
    return () => { listeners.delete(listener); };
  },
};
