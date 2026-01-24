<script lang="ts">
	import { goto } from '$app/navigation';
	import { base, resolve } from '$app/paths';
	import { authStore } from '$lib/stores/auth.svelte';
	import { auth, api } from '$lib/api/client';
	import { Button } from '$ui/button';
	import { Input } from '$ui/input';
	import { Label } from '$ui/label';
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

<div class="absolute left-6 top-6">
	<img
		src="{base}/alyx-icon.png"
		alt="Alyx"
		class="h-8 w-8 object-contain"
	/>
</div>

{#if isCheckingStatus}
	<div class="w-full max-w-[340px]">
		<div class="flex flex-col items-center justify-center py-12">
			<div class="h-5 w-5 animate-spin rounded-full border-2 border-foreground/20 border-t-foreground"></div>
		</div>
	</div>
{:else if needsSetup}
	<div class="w-full max-w-[340px]">
		<div class="mb-10 text-center">
			<h1 class="text-[28px] font-semibold leading-tight tracking-tight text-foreground">
				Create Admin Account
			</h1>
			<p class="mt-2 text-sm text-muted-foreground">
				Set up your first administrator account
			</p>
		</div>

		<form onsubmit={handleSetup} class="space-y-4">
			<div class="space-y-1.5">
				<Label for="email" class="text-sm font-normal text-foreground/70">Email Address</Label>
				<Input
					id="email"
					type="email"
					placeholder="admin@example.com"
					bind:value={email}
					required
					disabled={isLoading}
					class="h-10 rounded-md border-border/40 bg-transparent px-3 text-[15px] placeholder:text-muted-foreground/50 focus-visible:border-foreground/30 focus-visible:ring-0"
				/>
			</div>

			<div class="space-y-1.5">
				<Label for="password" class="text-sm font-normal text-foreground/70">Password</Label>
				<Input
					id="password"
					type="password"
					placeholder="Create a password"
					bind:value={password}
					required
					disabled={isLoading}
					class="h-10 rounded-md border-border/40 bg-transparent px-3 text-[15px] placeholder:text-muted-foreground/50 focus-visible:border-foreground/30 focus-visible:ring-0"
				/>
			</div>

			<div class="space-y-1.5">
				<Label for="confirmPassword" class="text-sm font-normal text-foreground/70">Confirm Password</Label>
				<Input
					id="confirmPassword"
					type="password"
					placeholder="Confirm your password"
					bind:value={confirmPassword}
					required
					disabled={isLoading}
					class="h-10 rounded-md border-border/40 bg-transparent px-3 text-[15px] placeholder:text-muted-foreground/50 focus-visible:border-foreground/30 focus-visible:ring-0"
				/>
			</div>

			<div class="pt-2">
				<Button 
					type="submit" 
					class="h-10 w-full rounded-md bg-foreground font-medium text-background hover:bg-foreground/90" 
					disabled={isLoading}
				>
					{isLoading ? 'Creating account...' : 'Create Admin Account'}
				</Button>
			</div>
		</form>
	</div>
{:else}
	<div class="w-full max-w-[340px]">
		<div class="mb-10 text-center">
			<h1 class="text-[28px] font-semibold leading-tight tracking-tight text-foreground">
				Log in to Alyx
			</h1>
		</div>

		<form onsubmit={handleLogin} class="space-y-4">
			<div class="space-y-1.5">
				<Label for="email" class="text-sm font-normal text-foreground/70">Email Address</Label>
				<Input
					id="email"
					type="email"
					placeholder="admin@example.com"
					bind:value={email}
					required
					disabled={isLoading}
					class="h-10 rounded-md border-border/40 bg-transparent px-3 text-[15px] placeholder:text-muted-foreground/50 focus-visible:border-foreground/30 focus-visible:ring-0"
				/>
			</div>

			<div class="space-y-1.5">
				<Label for="password" class="text-sm font-normal text-foreground/70">Password</Label>
				<Input
					id="password"
					type="password"
					placeholder="Enter your password"
					bind:value={password}
					required
					disabled={isLoading}
					class="h-10 rounded-md border-border/40 bg-transparent px-3 text-[15px] placeholder:text-muted-foreground/50 focus-visible:border-foreground/30 focus-visible:ring-0"
				/>
			</div>

			<div class="pt-2">
				<Button 
					type="submit" 
					class="h-10 w-full rounded-md bg-foreground font-medium text-background hover:bg-foreground/90" 
					disabled={isLoading}
				>
					{isLoading ? 'Signing in...' : 'Sign in'}
				</Button>
			</div>
		</form>
	</div>
{/if}
