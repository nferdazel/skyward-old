import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import LedgerTable from './LedgerTable.svelte';
import type { FinanceStore } from '../state/finance-store.svelte';
import type { BankTransaction } from '$lib/features/bank/domain/bank-models';

describe('LedgerTable component', () => {
	function createMockStore(transactions: BankTransaction[] = []) {
		return {
			state: {
				transactions,
				overview: null,
				loading: false,
				error: null
			}
		} as unknown as FinanceStore;
	}

	it('renders empty message when no transactions exist', () => {
		const store = createMockStore([]);
		render(LedgerTable, { props: { store } });
		expect(screen.getByText('No transactions.')).toBeInTheDocument();
	});

	it('renders transactions and slices to 50 per page with pagination bar', async () => {
		const manyTransactions: BankTransaction[] = Array.from({ length: 65 }, (_, i) => ({
			id: `tx-${i + 1}`,
			userId: 'user-1',
			accountId: 'acc-1',
			gameDate: '2026-10-20T12:00:00Z',
			transactionType: 'credit',
			category: 'revenue',
			ifrsCategory: 'operating',
			ifrsSubcategory: 'ticketSales',
			amount: 1000 + i,
			balanceAfter: 50000 + i * 1000,
			description: `Flight ticket sale #${i + 1}`
		}));

		const store = createMockStore(manyTransactions);
		render(LedgerTable, { props: { store } });

		// Should show pagination info
		expect(screen.getByText('Showing 1–50 of 65')).toBeInTheDocument();
		expect(screen.getByText('Flight ticket sale #1')).toBeInTheDocument();
		expect(screen.getByText('Flight ticket sale #50')).toBeInTheDocument();
		expect(screen.queryByText('Flight ticket sale #51')).not.toBeInTheDocument();

		// Click Next
		const nextBtn = screen.getByRole('button', { name: 'Next page' });
		expect(nextBtn).toBeEnabled();
		await fireEvent.click(nextBtn);

		// Now on page 2
		expect(screen.getByText('Showing 51–65 of 65')).toBeInTheDocument();
		expect(screen.getByText('Flight ticket sale #51')).toBeInTheDocument();
		expect(screen.queryByText('Flight ticket sale #1')).not.toBeInTheDocument();
	});
});
