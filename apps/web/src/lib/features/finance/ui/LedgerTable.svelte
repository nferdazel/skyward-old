<script lang="ts">
	import CraftCard from '$lib/core/components/CraftCard.svelte';
	import AppBadge from '$lib/core/components/AppBadge.svelte';
	import type { FinanceStore } from '../state/finance-store.svelte';
	import { groupFor, type IfrsGroup } from '../domain/ifrs-category';
	import type { BankTransaction } from '$lib/features/bank/domain/bank-models';

	/**
	 * Ledger tab. Ported from Flutter `FinanceLedgerFilters` + transactions table:
	 * a filter strip by IFRS group over the transaction list.
	 */
	type Props = { store: FinanceStore };
	let { store }: Props = $props();

	type Filter = 'all' | IfrsGroup;
	let filter = $state<Filter>('all');
	let currentPage = $state(1);

	const fmt = new Intl.NumberFormat('en-US', { maximumFractionDigits: 0 });
	const money = (n: number) => `$${fmt.format(Math.round(n))}`;

	function groupOf(t: BankTransaction): IfrsGroup {
		const key = t.ifrsSubcategory || t.ifrsCategory || '';
		return groupFor(key);
	}

	const filters: { value: Filter; label: string }[] = [
		{ value: 'all', label: 'All' },
		{ value: 'ticketSales', label: 'Revenue' },
		{ value: 'operations', label: 'Operations' },
		{ value: 'lease', label: 'Lease' },
		{ value: 'repair', label: 'Repairs' },
		{ value: 'purchase', label: 'Acquisition' },
		{ value: 'financing', label: 'Financing' }
	];

	const filtered = $derived(
		filter === 'all'
			? store.state.transactions
			: store.state.transactions.filter((t) => groupOf(t) === filter)
	);

	const PAGE_SIZE = 50;
	const totalPages = $derived(Math.max(1, Math.ceil(filtered.length / PAGE_SIZE)));
	const safePage = $derived(Math.min(Math.max(1, currentPage), totalPages));
	const startIndex = $derived((safePage - 1) * PAGE_SIZE);
	const endIndex = $derived(Math.min(startIndex + PAGE_SIZE, filtered.length));
	const paginated = $derived(filtered.slice(startIndex, endIndex));

	function setFilter(f: Filter) {
		filter = f;
		currentPage = 1;
	}

	function tone(t: BankTransaction): 'success' | 'error' {
		return t.transactionType === 'credit' ? 'success' : 'error';
	}
</script>

<div class="ledger">
	<div class="filters">
		{#each filters as f (f.value)}
			<button class:active={filter === f.value} onclick={() => setFilter(f.value)}>{f.label}</button
			>
		{/each}
	</div>

	<CraftCard>
		{#if filtered.length === 0}
			<p class="muted">No transactions.</p>
		{:else}
			<div class="table-wrap">
				<table>
					<thead>
						<tr>
							<th>Game date</th>
							<th>Description</th>
							<th>Category</th>
							<th class="right">Amount</th>
							<th class="right">Balance</th>
						</tr>
					</thead>
					<tbody>
						{#each paginated as t (t.id)}
							<tr>
								<td class="muted">{t.gameDate ? t.gameDate.slice(0, 10) : '—'}</td>
								<td>{t.description ?? t.transactionType}</td>
								<td><AppBadge label={t.ifrsCategory ?? 'other'} tone={tone(t)} /></td>
								<td
									class="right tnum"
									style="color: {tone(t) === 'success'
										? 'var(--color-success)'
										: 'var(--color-error)'};"
								>
									{tone(t) === 'success' ? '+' : '-'}{money(Math.abs(t.amount))}
								</td>
								<td class="right tnum muted">{money(t.balanceAfter)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>

			{#if filtered.length > PAGE_SIZE}
				<div class="pagination-bar">
					<span class="pagination-info muted tnum">
						Showing {startIndex + 1}–{endIndex} of {filtered.length}
					</span>
					<div class="pagination-actions">
						<button
							class="page-btn"
							disabled={safePage <= 1}
							onclick={() => (currentPage = safePage - 1)}
							aria-label="Previous page"
						>
							Prev
						</button>
						<span class="page-indicator tnum muted">
							{safePage} / {totalPages}
						</span>
						<button
							class="page-btn"
							disabled={safePage >= totalPages}
							onclick={() => (currentPage = safePage + 1)}
							aria-label="Next page"
						>
							Next
						</button>
					</div>
				</div>
			{/if}
		{/if}
	</CraftCard>
</div>

<style>
	.ledger {
		display: flex;
		flex-direction: column;
		gap: var(--space-md);
	}
	.filters {
		display: flex;
		gap: var(--space-xs);
		flex-wrap: wrap;
	}
	.filters button {
		padding: var(--space-xs) var(--space-sm);
		background: var(--color-surface);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-default);
		color: var(--color-text-secondary);
		font-size: 11px;
		cursor: pointer;
	}
	.filters button.active {
		background: var(--color-accent-subtle);
		border-color: var(--color-accent);
		color: var(--color-accent);
	}
	.table-wrap {
		overflow-x: auto;
	}
	table {
		width: 100%;
		border-collapse: collapse;
		font-size: 13px;
	}
	th {
		text-align: left;
		padding: var(--space-sm);
		color: var(--color-text-secondary);
		font-size: 10px;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		border-bottom: 1px solid var(--color-border);
	}
	td {
		padding: var(--space-sm);
		border-bottom: 0.5px solid var(--color-border-subtle);
	}
	.right {
		text-align: right;
	}
	.muted {
		color: var(--color-text-muted);
		font-size: 12px;
	}
	.pagination-bar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding-top: var(--space-sm);
		margin-top: var(--space-sm);
		border-top: 1px solid var(--color-border);
		flex-wrap: wrap;
		gap: var(--space-xs);
	}
	.pagination-actions {
		display: flex;
		align-items: center;
		gap: var(--space-xs);
	}
	.page-btn {
		padding: 4px var(--space-sm);
		background: var(--color-surface);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-sm);
		color: var(--color-text-primary);
		font-size: 11px;
		font-family: inherit;
		cursor: pointer;
		min-height: 28px;
		min-width: 48px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		transition:
			background var(--transition-fast),
			border-color var(--transition-fast);
	}
	.page-btn:hover:not(:disabled) {
		background: var(--color-surface-hover);
		border-color: var(--color-border-strong);
	}
	.page-btn:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}
	.page-indicator {
		font-size: 11px;
		padding: 0 var(--space-xs);
	}
</style>
