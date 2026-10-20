/**
 * Svelte action that traps keyboard Tab navigation within a modal or drawer.
 * Restores focus to the previously focused element upon unmounting.
 */
export function focusTrap(node: HTMLElement) {
	const previouslyFocused = document.activeElement as HTMLElement | null;

	const focusableSelector =
		'a[href], button:not([disabled]):not([tabindex="-1"]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])';

	function getFocusableElements(): HTMLElement[] {
		return Array.from(node.querySelectorAll<HTMLElement>(focusableSelector)).filter(
			(el) => el.offsetParent !== null && !el.hasAttribute('disabled')
		);
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key !== 'Tab') return;

		const focusable = getFocusableElements();
		if (focusable.length === 0) {
			e.preventDefault();
			return;
		}

		const first = focusable[0];
		const last = focusable[focusable.length - 1];

		if (e.shiftKey) {
			if (document.activeElement === first || !node.contains(document.activeElement)) {
				e.preventDefault();
				last.focus();
			}
		} else {
			if (document.activeElement === last || !node.contains(document.activeElement)) {
				e.preventDefault();
				first.focus();
			}
		}
	}

	// Focus first interactive element once rendered
	const timer = setTimeout(() => {
		const focusable = getFocusableElements();
		if (focusable.length > 0 && !node.contains(document.activeElement)) {
			focusable[0].focus();
		}
	}, 20);

	node.addEventListener('keydown', handleKeydown);

	return {
		destroy() {
			clearTimeout(timer);
			node.removeEventListener('keydown', handleKeydown);
			if (previouslyFocused && typeof previouslyFocused.focus === 'function') {
				previouslyFocused.focus();
			}
		}
	};
}
