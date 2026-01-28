<script lang="ts">
	import { createQuery, createMutation, useQueryClient } from '@tanstack/svelte-query';
	import { admin, type User } from '$lib/api/client';
	import * as Card from '$ui/card';
	import * as Table from '$ui/table';
	import * as Dialog from '$ui/dialog';
	import * as AlertDialog from '$ui/alert-dialog';
	import { Skeleton } from '$ui/skeleton';
	import { Button } from '$ui/button';
	import { Input } from '$ui/input';
	import { Badge } from '$ui/badge';
	import { toast } from 'svelte-sonner';

	import PlusIcon from 'lucide-svelte/icons/plus';
	import SearchIcon from 'lucide-svelte/icons/search';
	import Trash2Icon from 'lucide-svelte/icons/trash-2';
	import PencilIcon from 'lucide-svelte/icons/pencil';
	import KeyIcon from 'lucide-svelte/icons/key';
	import Loader2Icon from 'lucide-svelte/icons/loader-2';
	import ChevronLeftIcon from 'lucide-svelte/icons/chevron-left';
	import ChevronRightIcon from 'lucide-svelte/icons/chevron-right';
	import ChevronsLeftIcon from 'lucide-svelte/icons/chevrons-left';
	import ChevronsRightIcon from 'lucide-svelte/icons/chevrons-right';

	const queryClient = useQueryClient();

	// Local definition to avoid import issues reported by LSP
	interface ApiError {
		code: string;
		error: string;
		message: string;
		details?: { field: string; code: string; message: string }[];
	}

	let pageIndex = $state(1);
	let pageSize = $state(10);
	let search = $state('');
	let roleFilter = $state<string>('');

	let isCreateOpen = $state(false);
	let isEditOpen = $state(false);
	let isPasswordOpen = $state(false);
	let itemToDelete = $state<User | null>(null);
	let selectedUser = $state<User | null>(null);

	let createForm = $state({ email: '', password: '', role: 'user', verified: false });
	let editForm = $state({ email: '', role: 'user', verified: false });
	let passwordForm = $state({ password: '' });

	function getErrorMessage(err: unknown): string {
		if (err && typeof err === 'object' && 'error' in err) {
			const apiErr = err as ApiError;
			return apiErr.error || apiErr.message || 'Unknown error';
		}
		if (err instanceof Error) {
			return err.message;
		}
		return 'Unknown error';
	}

	function resetForms() {
		createForm = { email: '', password: '', role: 'user', verified: false };
		editForm = { email: '', role: 'user', verified: false };
		passwordForm = { password: '' };
		selectedUser = null;
	}

	const usersQuery = createQuery(() => ({
		queryKey: ['users', pageIndex, pageSize, search, roleFilter],
		queryFn: async () => {
			const result = await admin.users.list({
				limit: pageSize,
				offset: (pageIndex - 1) * pageSize,
				search: search || undefined,
				role: roleFilter || undefined,
				sort_by: 'created_at',
				sort_dir: 'desc'
			});
			if (result.error) throw new Error(result.error.message);
			return result.data!;
		}
	}));

	const totalPages = $derived(Math.max(1, Math.ceil((usersQuery.data?.total || 0) / pageSize)));

	const createUserMutation = createMutation(() => ({
		mutationFn: async (data: typeof createForm) => {
			const result = await admin.users.create(data);
			if (result.error) throw result.error;
			return result.data;
		},
		onSuccess: () => {
			toast.success('User created');
			queryClient.invalidateQueries({ queryKey: ['users'] });
			isCreateOpen = false;
			resetForms();
		},
		onError: (err) => {
			toast.error('Failed to create user', { description: getErrorMessage(err) });
		}
	}));

	const updateUserMutation = createMutation(() => ({
		mutationFn: async (data: { id: string; email: string; role: string; verified: boolean }) => {
			const result = await admin.users.update(data.id, {
				email: data.email,
				role: data.role,
				verified: data.verified
			});
			if (result.error) throw result.error;
			return result.data;
		},
		onSuccess: () => {
			toast.success('User updated');
			queryClient.invalidateQueries({ queryKey: ['users'] });
			isEditOpen = false;
			resetForms();
		},
		onError: (err) => {
			toast.error('Failed to update user', { description: getErrorMessage(err) });
		}
	}));

	const deleteUserMutation = createMutation(() => ({
		mutationFn: async (id: string) => {
			const result = await admin.users.delete(id);
			if (result.error) throw result.error;
			return id;
		},
		onSuccess: () => {
			toast.success('User deleted');
			queryClient.invalidateQueries({ queryKey: ['users'] });
			itemToDelete = null;
		},
		onError: (err) => {
			toast.error('Failed to delete user', { description: getErrorMessage(err) });
		}
	}));

	const setPasswordMutation = createMutation(() => ({
		mutationFn: async (data: { id: string; password: string }) => {
			const result = await admin.users.setPassword(data.id, data.password);
			if (result.error) throw result.error;
			return result.data;
		},
		onSuccess: () => {
			toast.success('Password updated');
			isPasswordOpen = false;
			resetForms();
		},
		onError: (err) => {
			toast.error('Failed to set password', { description: getErrorMessage(err) });
		}
	}));

	function openEdit(user: User) {
		selectedUser = user;
		editForm = {
			email: user.email,
			role: user.role,
			verified: user.verified
		};
		isEditOpen = true;
	}

	function openPassword(user: User) {
		selectedUser = user;
		passwordForm = { password: '' };
		isPasswordOpen = true;
	}
</script>

<div class="max-w-screen-2xl mx-auto space-y-6">
	<div class="flex items-center justify-between mb-6">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">Users</h1>
			<p class="text-sm text-muted-foreground">
				{#if usersQuery.data}
					{usersQuery.data.total} user{usersQuery.data.total === 1 ? '' : 's'}
				{:else}
					Loading...
				{/if}
			</p>
		</div>
		<Button onclick={() => (isCreateOpen = true)}>
			<PlusIcon class="h-4 w-4 mr-2" />
			Create User
		</Button>
	</div>

	<div class="flex items-center gap-4 mb-4">
		<div class="relative flex-1 max-w-sm">
			<SearchIcon class="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
			<Input
				type="search"
				placeholder="Search by email..."
				class="pl-9"
				value={search}
				oninput={(e) => {
					search = e.currentTarget.value;
					pageIndex = 1;
				}}
			/>
		</div>
		<select
			class="h-10 rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
			value={roleFilter}
			onchange={(e) => {
				roleFilter = e.currentTarget.value;
				pageIndex = 1;
			}}
		>
			<option value="">All Roles</option>
			<option value="user">User</option>
			<option value="admin">Admin</option>
		</select>
	</div>

	{#if usersQuery.isPending}
		<Card.Root class="flex items-center justify-center py-16">
			<Skeleton class="h-32 w-32" />
		</Card.Root>
	{:else if usersQuery.isError}
		<Card.Root class="border-destructive/50 flex items-center justify-center py-16">
			<p class="text-sm text-destructive">
				Failed to load users: {usersQuery.error?.message}
			</p>
		</Card.Root>
	{:else if usersQuery.data?.users.length === 0}
		<div class="flex flex-col items-center justify-center py-16 text-center rounded-md border">
			<div class="rounded-full bg-muted p-4 mb-4">
				<svg
					xmlns="http://www.w3.org/2000/svg"
					class="h-8 w-8 text-muted-foreground"
					fill="none"
					viewBox="0 0 24 24"
					stroke="currentColor"
				>
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						stroke-width="2"
						d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"
					/>
				</svg>
			</div>
			<h3 class="text-sm font-medium mb-1">No users yet</h3>
			<p class="text-sm text-muted-foreground">Get started by creating your first user</p>
		</div>
	{:else}
		<div class="rounded-md border overflow-hidden">
			<Table.Root>
				<Table.Header class="sticky top-0 z-20 bg-card">
					<Table.Row>
						<Table.Head>Email</Table.Head>
						<Table.Head>Role</Table.Head>
						<Table.Head>Status</Table.Head>
						<Table.Head>Created</Table.Head>
						<Table.Head class="w-[60px]">
							<span class="sr-only">Actions</span>
						</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each usersQuery.data?.users ?? [] as user (user.id)}
						<Table.Row>
							<Table.Cell class="font-medium">{user.email}</Table.Cell>
							<Table.Cell>
								<Badge variant={user.role === 'admin' ? 'default' : 'secondary'}>
									{user.role}
								</Badge>
							</Table.Cell>
							<Table.Cell>
								{#if user.verified}
									<Badge variant="outline" class="border-green-500 text-green-500">Verified</Badge>
								{:else}
									<Badge variant="outline" class="text-muted-foreground">Unverified</Badge>
								{/if}
							</Table.Cell>
							<Table.Cell class="text-muted-foreground">
								{new Date(user.created_at).toLocaleDateString()}
							</Table.Cell>
							<Table.Cell>
								<div class="flex items-center justify-end gap-2">
									<Button
										variant="ghost"
										size="icon"
										class="h-8 w-8"
										title="Edit User"
										onclick={() => openEdit(user)}
									>
										<PencilIcon class="h-4 w-4" />
									</Button>
									<Button
										variant="ghost"
										size="icon"
										class="h-8 w-8"
										title="Set Password"
										onclick={() => openPassword(user)}
									>
										<KeyIcon class="h-4 w-4" />
									</Button>
									<Button
										variant="ghost"
										size="icon"
										class="h-8 w-8 text-destructive hover:text-destructive"
										title="Delete User"
										onclick={() => (itemToDelete = user)}
									>
										<Trash2Icon class="h-4 w-4" />
									</Button>
								</div>
							</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</div>

		<div class="flex items-center justify-between mt-4">
			<div class="flex items-center gap-2 text-sm text-muted-foreground">
				<span>Rows per page</span>
				<select
					class="h-8 w-[70px] rounded-md border border-input bg-background px-2 py-1 text-sm"
					value={String(pageSize)}
					onchange={(e) => {
						pageSize = Number(e.currentTarget.value);
						pageIndex = 1;
					}}
				>
					<option value="10">10</option>
					<option value="25">25</option>
					<option value="50">50</option>
					<option value="100">100</option>
				</select>
			</div>

			<div class="flex items-center gap-4">
				<span class="text-sm text-muted-foreground">
					Page {pageIndex} of {totalPages}
				</span>
				<div class="flex items-center gap-1">
					<Button
						variant="outline"
						size="icon"
						class="h-8 w-8"
						onclick={() => (pageIndex = 1)}
						disabled={pageIndex === 1}
					>
						<ChevronsLeftIcon class="h-4 w-4" />
					</Button>
					<Button
						variant="outline"
						size="icon"
						class="h-8 w-8"
						onclick={() => pageIndex--}
						disabled={pageIndex === 1}
					>
						<ChevronLeftIcon class="h-4 w-4" />
					</Button>
					<Button
						variant="outline"
						size="icon"
						class="h-8 w-8"
						onclick={() => pageIndex++}
						disabled={pageIndex >= totalPages}
					>
						<ChevronRightIcon class="h-4 w-4" />
					</Button>
					<Button
						variant="outline"
						size="icon"
						class="h-8 w-8"
						onclick={() => (pageIndex = totalPages)}
						disabled={pageIndex >= totalPages}
					>
						<ChevronsRightIcon class="h-4 w-4" />
					</Button>
				</div>
			</div>
		</div>
	{/if}

	<Dialog.Root bind:open={isCreateOpen}>
		<Dialog.Content>
			<Dialog.Header>
				<Dialog.Title>Create User</Dialog.Title>
				<Dialog.Description>Add a new user to the system.</Dialog.Description>
			</Dialog.Header>
			<div class="grid gap-4 py-4">
				<div class="grid gap-2">
					<label for="create-email" class="text-sm font-medium">Email</label>
					<Input id="create-email" type="email" bind:value={createForm.email} placeholder="user@example.com" />
				</div>
				<div class="grid gap-2">
					<label for="create-password" class="text-sm font-medium">Password</label>
					<Input id="create-password" type="password" bind:value={createForm.password} placeholder="••••••••" />
				</div>
				<div class="grid gap-2">
					<label for="create-role" class="text-sm font-medium">Role</label>
					<select
						id="create-role"
						class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
						bind:value={createForm.role}
					>
						<option value="user">User</option>
						<option value="admin">Admin</option>
					</select>
				</div>
				<div class="flex items-center gap-2">
					<input
						id="create-verified"
						type="checkbox"
						class="h-4 w-4 rounded border-input"
						bind:checked={createForm.verified}
					/>
					<label for="create-verified" class="text-sm font-medium">Verified</label>
				</div>
			</div>
			<Dialog.Footer>
				<Button variant="outline" onclick={() => (isCreateOpen = false)}>Cancel</Button>
				<Button
					onclick={() => createUserMutation.mutate(createForm)}
					disabled={createUserMutation.isPending || !createForm.email || !createForm.password}
				>
					{#if createUserMutation.isPending}
						<Loader2Icon class="mr-2 h-4 w-4 animate-spin" />
					{/if}
					Create
				</Button>
			</Dialog.Footer>
		</Dialog.Content>
	</Dialog.Root>

	<Dialog.Root bind:open={isEditOpen}>
		<Dialog.Content>
			<Dialog.Header>
				<Dialog.Title>Edit User</Dialog.Title>
				<Dialog.Description>Update user details.</Dialog.Description>
			</Dialog.Header>
			<div class="grid gap-4 py-4">
				<div class="grid gap-2">
					<label for="edit-email" class="text-sm font-medium">Email</label>
					<Input id="edit-email" type="email" bind:value={editForm.email} />
				</div>
				<div class="grid gap-2">
					<label for="edit-role" class="text-sm font-medium">Role</label>
					<select
						id="edit-role"
						class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
						bind:value={editForm.role}
					>
						<option value="user">User</option>
						<option value="admin">Admin</option>
					</select>
				</div>
				<div class="flex items-center gap-2">
					<input
						id="edit-verified"
						type="checkbox"
						class="h-4 w-4 rounded border-input"
						bind:checked={editForm.verified}
					/>
					<label for="edit-verified" class="text-sm font-medium">Verified</label>
				</div>
			</div>
			<Dialog.Footer>
				<Button variant="outline" onclick={() => (isEditOpen = false)}>Cancel</Button>
				<Button
					onclick={() => selectedUser && updateUserMutation.mutate({ id: selectedUser.id, ...editForm })}
					disabled={updateUserMutation.isPending || !editForm.email}
				>
					{#if updateUserMutation.isPending}
						<Loader2Icon class="mr-2 h-4 w-4 animate-spin" />
					{/if}
					Save Changes
				</Button>
			</Dialog.Footer>
		</Dialog.Content>
	</Dialog.Root>

	<Dialog.Root bind:open={isPasswordOpen}>
		<Dialog.Content>
			<Dialog.Header>
				<Dialog.Title>Set Password</Dialog.Title>
				<Dialog.Description>
					Set a new password for {selectedUser?.email}.
				</Dialog.Description>
			</Dialog.Header>
			<div class="grid gap-4 py-4">
				<div class="grid gap-2">
					<label for="new-password" class="text-sm font-medium">New Password</label>
					<Input id="new-password" type="password" bind:value={passwordForm.password} placeholder="••••••••" />
				</div>
			</div>
			<Dialog.Footer>
				<Button variant="outline" onclick={() => (isPasswordOpen = false)}>Cancel</Button>
				<Button
					onclick={() => selectedUser && setPasswordMutation.mutate({ id: selectedUser.id, password: passwordForm.password })}
					disabled={setPasswordMutation.isPending || !passwordForm.password}
				>
					{#if setPasswordMutation.isPending}
						<Loader2Icon class="mr-2 h-4 w-4 animate-spin" />
					{/if}
					Set Password
				</Button>
			</Dialog.Footer>
		</Dialog.Content>
	</Dialog.Root>

	<AlertDialog.Root open={!!itemToDelete} onOpenChange={(open) => !open && (itemToDelete = null)}>
		<AlertDialog.Content>
			<AlertDialog.Header>
				<AlertDialog.Title>Delete user?</AlertDialog.Title>
				<AlertDialog.Description>
					This action cannot be undone. This will permanently delete the user
					<span class="font-medium text-foreground">{itemToDelete?.email}</span>.
				</AlertDialog.Description>
			</AlertDialog.Header>
			<AlertDialog.Footer>
				<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
				<AlertDialog.Action
					class="bg-destructive text-destructive-foreground hover:bg-destructive/90"
					onclick={() => itemToDelete && deleteUserMutation.mutate(itemToDelete.id)}
					disabled={deleteUserMutation.isPending}
				>
					{#if deleteUserMutation.isPending}
						<Loader2Icon class="mr-2 h-4 w-4 animate-spin" />
					{/if}
					Delete
				</AlertDialog.Action>
			</AlertDialog.Footer>
		</AlertDialog.Content>
	</AlertDialog.Root>
</div>
