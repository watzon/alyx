<script lang="ts">
	import { goto } from '$app/navigation';
	import { base, resolve } from '$app/paths';
	import { authStore } from '$lib/stores/auth.svelte';
	import { auth, api } from '$lib/api/client';
	import { Button } from '$ui/button';
	import { Input } from '$ui/input';
	import { Label } from '$ui/label';
	import * as Card from '$ui/card';
	import { toast } from 'svelte-sonner';
	import { onMount } from 'svelte';

	let email = $state('');
	let password = $state('');
	let confirmPassword = $state('');
	let isLoading = $state(false);
	let isCheckingStatus = $state(true);
	let needsSetup = $state(false);

	onMount(async () => {
		try {
			const result = await auth.status();
			if (result.data) {
				needsSetup = result.data.needs_setup;
			}
		} catch {
			// If status check fails, default to login mode
			needsSetup = false;
		} finally {
			isCheckingStatus = false;
		}
	});

	async function handleLogin(e: Event) {
		e.preventDefault();
		isLoading = true;

		try {
			await authStore.login(email, password);
			toast.success('Logged in successfully');
			goto(resolve('/(app)'));
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Login failed');
		} finally {
			isLoading = false;
		}
	}

	async function handleSetup(e: Event) {
		e.preventDefault();

		if (password !== confirmPassword) {
			toast.error('Passwords do not match');
			return;
		}

		isLoading = true;

		try {
			const result = await auth.register(email, password);
			if (result.error) {
				toast.error(result.error.message || 'Registration failed');
				return;
			}
			if (result.data?.tokens) {
				api.setToken(result.data.tokens.access_token);
				api.setRefreshToken(result.data.tokens.refresh_token);
				toast.success('Admin account created successfully');
				goto(resolve('/(app)'));
			}
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Registration failed');
		} finally {
			isLoading = false;
		}
	}
</script>

{#if isCheckingStatus}
	<Card.Root class="w-full max-w-sm border-border/50 shadow-2xl shadow-black/50">
		<Card.Header class="pb-4">
			<div class="flex flex-col items-center gap-3 pb-2">
				<div class="flex h-16 w-16 items-center justify-center rounded-xl bg-gradient-to-br from-primary/20 to-primary/5 p-2 ring-1 ring-primary/20">
					<img
						src="{base}/alyx-icon.png"
						alt="Alyx"
						class="h-10 w-10 object-contain"
					/>
				</div>
				<div class="text-center">
					<h1 class="text-xl font-semibold tracking-tight">Alyx Admin</h1>
					<p class="text-sm text-muted-foreground mt-1">Backend-as-a-Service</p>
				</div>
			</div>
		</Card.Header>
		<Card.Content class="flex items-center justify-center py-8">
			<div class="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
		</Card.Content>
	</Card.Root>
{:else if needsSetup}
	<Card.Root class="w-full max-w-sm border-border/50 shadow-2xl shadow-black/50">
		<Card.Header class="pb-4">
			<div class="flex flex-col items-center gap-3 pb-2">
				<div class="flex h-16 w-16 items-center justify-center rounded-xl bg-gradient-to-br from-primary/20 to-primary/5 p-2 ring-1 ring-primary/20">
					<img
						src="{base}/alyx-icon.png"
						alt="Alyx"
						class="h-10 w-10 object-contain"
					/>
				</div>
				<div class="text-center">
					<h1 class="text-xl font-semibold tracking-tight">Alyx Admin</h1>
					<p class="text-sm text-muted-foreground mt-1">Backend-as-a-Service</p>
				</div>
			</div>
			<div class="pt-4 border-t border-border/50">
				<Card.Title class="text-lg font-medium">Create Admin Account</Card.Title>
				<Card.Description class="text-muted-foreground">
					Set up your first administrator account
				</Card.Description>
			</div>
		</Card.Header>
		<form onsubmit={handleSetup}>
			<Card.Content class="space-y-4 pb-6">
				<div class="space-y-2">
					<Label for="email" class="text-sm font-medium">Email</Label>
					<Input
						id="email"
						type="email"
						placeholder="admin@example.com"
						bind:value={email}
						required
						disabled={isLoading}
						class="h-10 bg-muted/50 border-border/50 focus-visible:ring-primary/50"
					/>
				</div>
				<div class="space-y-2">
					<Label for="password" class="text-sm font-medium">Password</Label>
					<Input
						id="password"
						type="password"
						placeholder="Create a password"
						bind:value={password}
						required
						disabled={isLoading}
						class="h-10 bg-muted/50 border-border/50 focus-visible:ring-primary/50"
					/>
				</div>
				<div class="space-y-2">
					<Label for="confirmPassword" class="text-sm font-medium">Confirm Password</Label>
					<Input
						id="confirmPassword"
						type="password"
						placeholder="Confirm your password"
						bind:value={confirmPassword}
						required
						disabled={isLoading}
						class="h-10 bg-muted/50 border-border/50 focus-visible:ring-primary/50"
					/>
				</div>
			</Card.Content>
			<Card.Footer class="pt-0">
				<Button type="submit" class="w-full h-10 font-medium" disabled={isLoading}>
					{isLoading ? 'Creating account...' : 'Create Admin Account'}
				</Button>
			</Card.Footer>
		</form>
	</Card.Root>
{:else}
	<Card.Root class="w-full max-w-sm border-border/50 shadow-2xl shadow-black/50">
		<Card.Header class="pb-4">
			<div class="flex flex-col items-center gap-3 pb-2">
				<div class="flex h-16 w-16 items-center justify-center rounded-xl bg-gradient-to-br from-primary/20 to-primary/5 p-2 ring-1 ring-primary/20">
					<img
						src="{base}/alyx-icon.png"
						alt="Alyx"
						class="h-10 w-10 object-contain"
					/>
				</div>
				<div class="text-center">
					<h1 class="text-xl font-semibold tracking-tight">Alyx Admin</h1>
					<p class="text-sm text-muted-foreground mt-1">Backend-as-a-Service</p>
				</div>
			</div>
			<div class="pt-4 border-t border-border/50">
				<Card.Title class="text-lg font-medium">Sign in</Card.Title>
				<Card.Description class="text-muted-foreground">
					Enter your credentials to continue
				</Card.Description>
			</div>
		</Card.Header>
		<form onsubmit={handleLogin}>
			<Card.Content class="space-y-4 pb-6">
				<div class="space-y-2">
					<Label for="email" class="text-sm font-medium">Email</Label>
					<Input
						id="email"
						type="email"
						placeholder="admin@example.com"
						bind:value={email}
						required
						disabled={isLoading}
						class="h-10 bg-muted/50 border-border/50 focus-visible:ring-primary/50"
					/>
				</div>
				<div class="space-y-2">
					<Label for="password" class="text-sm font-medium">Password</Label>
					<Input
						id="password"
						type="password"
						placeholder="Enter your password"
						bind:value={password}
						required
						disabled={isLoading}
						class="h-10 bg-muted/50 border-border/50 focus-visible:ring-primary/50"
					/>
				</div>
			</Card.Content>
			<Card.Footer class="pt-0">
				<Button type="submit" class="w-full h-10 font-medium" disabled={isLoading}>
					{isLoading ? 'Signing in...' : 'Sign in'}
				</Button>
			</Card.Footer>
		</form>
	</Card.Root>
{/if}
