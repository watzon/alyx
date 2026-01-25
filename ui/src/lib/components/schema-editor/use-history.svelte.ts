import type { EditableSchema } from './types';
import { untrack } from 'svelte';

const MAX_HISTORY_SIZE = 50;

export interface HistoryState {
	past: EditableSchema[];
	present: EditableSchema;
	future: EditableSchema[];
}

function deepClone<T>(obj: T): T {
	return JSON.parse(JSON.stringify(obj));
}

export function createHistory(initial: EditableSchema) {
	let past = $state<EditableSchema[]>([]);
	let present = $state<EditableSchema>(deepClone(initial));
	let future = $state<EditableSchema[]>([]);

	function push(newState: EditableSchema) {
		untrack(() => {
			past = [...past.slice(-(MAX_HISTORY_SIZE - 1)), deepClone(present)];
			present = deepClone(newState);
			future = [];
		});
	}

	function undo(): EditableSchema | null {
		return untrack(() => {
			if (past.length === 0) return null;

			const previous = past[past.length - 1];
			past = past.slice(0, -1);
			future = [deepClone(present), ...future.slice(0, MAX_HISTORY_SIZE - 1)];
			present = previous;
			return deepClone(present);
		});
	}

	function redo(): EditableSchema | null {
		return untrack(() => {
			if (future.length === 0) return null;

			const next = future[0];
			future = future.slice(1);
			past = [...past.slice(-(MAX_HISTORY_SIZE - 1)), deepClone(present)];
			present = next;
			return deepClone(present);
		});
	}

	function reset(newState: EditableSchema) {
		untrack(() => {
			past = [];
			present = deepClone(newState);
			future = [];
		});
	}

	return {
		get canUndo() {
			return past.length > 0;
		},
		get canRedo() {
			return future.length > 0;
		},
		get current() {
			return present;
		},
		push,
		undo,
		redo,
		reset
	};
}

export type History = ReturnType<typeof createHistory>;
