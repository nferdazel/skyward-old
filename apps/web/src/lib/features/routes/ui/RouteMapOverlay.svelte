<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import 'leaflet/dist/leaflet.css';
	import type { Airport } from '../domain/airport';
	import type { UserRoute } from '../domain/route-models';

	/**
	 * Full-screen route map (Leaflet, dark CARTO tiles). Ported from the Flutter
	 * `_buildFullMap` layer: draws airports + route arcs. Presentation only — no
	 * economics here.
	 *
	 * Leaflet JS is dynamically imported so the static build has no Node/DOM
	 * dependency at build time; its CSS is imported statically so it is always
	 * bundled and present before the map initialises (a dynamically injected
	 * stylesheet can arrive after L.map() and leave the container unsized).
	 */
	type Props = {
		airports: Airport[];
		routes: UserRoute[];
		/** Optional planned leg (origin→destination) drawn as a preview line. */
		preview?: { origin: Airport; destination: Airport } | null;
		onmaptap?: (airport: Airport) => void;
	};

	let { airports, routes, preview = null, onmaptap }: Props = $props();
	let container: HTMLDivElement | undefined = $state();
	let cleanup: (() => void) | undefined;
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	let Lmod = $state<any>();
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	let map = $state<any>();
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	let markersLayer: any;
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	let routesLayer: any;
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	let previewLayer: any;

	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	type L = any;

	function greatCircle(a: Airport, b: Airport, steps: number): [number, number][] {
		const toRad = (d: number) => (d * Math.PI) / 180;
		const toDeg = (r: number) => (r * 180) / Math.PI;
		const lat1 = toRad(a.latitude);
		const lon1 = toRad(a.longitude);
		const lat2 = toRad(b.latitude);
		const lon2 = toRad(b.longitude);
		const d =
			2 *
			Math.asin(
				Math.sqrt(
					Math.sin((lat2 - lat1) / 2) ** 2 +
						Math.cos(lat1) * Math.cos(lat2) * Math.sin((lon2 - lon1) / 2) ** 2
				)
			);
		if (d === 0) return [];
		const out: [number, number][] = [];
		for (let i = 1; i < steps; i++) {
			const f = i / steps;
			const A = Math.sin((1 - f) * d) / Math.sin(d);
			const B = Math.sin(f * d) / Math.sin(d);
			const x = A * Math.cos(lat1) * Math.cos(lon1) + B * Math.cos(lat2) * Math.cos(lon2);
			const y = A * Math.cos(lat1) * Math.sin(lon1) + B * Math.cos(lat2) * Math.sin(lon2);
			const z = A * Math.sin(lat1) + B * Math.sin(lat2);
			out.push([toDeg(Math.atan2(z, Math.sqrt(x * x + y * y))), toDeg(Math.atan2(y, x))]);
		}
		return out;
	}

	$effect(() => {
		if (!map || !Lmod || !markersLayer || !routesLayer || !previewLayer) return;

		markersLayer.clearLayers();
		routesLayer.clearLayers();
		previewLayer.clearLayers();

		const byIata = new Map(airports.map((a) => [a.iata, a]));
		const connected = new Set<string>();
		for (const r of routes) {
			connected.add(r.originIata);
			connected.add(r.destinationIata);
		}

		// Draw all airports: high contrast for connected routes, subtle cockpit dots for unconnected
		for (const a of airports) {
			const isConnected = connected.has(a.iata);
			const marker = Lmod.circleMarker([a.latitude, a.longitude], {
				radius: isConnected ? 6 : 3.5,
				color: isConnected ? '#5B9EE0' : 'rgba(255, 255, 255, 0.35)',
				fillColor: isConnected ? '#5B9EE0' : '#161C25',
				fillOpacity: isConnected ? 0.9 : 0.6,
				weight: isConnected ? 1.5 : 1
			}).addTo(markersLayer);

			marker.bindTooltip(`[${a.iata}] ${a.city} (${a.country})`);
			if (onmaptap) marker.on('click', () => onmaptap(a));
		}

		for (const r of routes) {
			const o = byIata.get(r.originIata);
			const d = byIata.get(r.destinationIata);
			if (!o || !d) continue;
			const arc = greatCircle(o, d, 18);
			Lmod.polyline([[o.latitude, o.longitude], ...arc, [d.latitude, d.longitude]], {
				color: '#3AAFA0',
				weight: 2.5,
				opacity: 0.72
			}).addTo(routesLayer);
		}

		if (preview) {
			const arc = greatCircle(preview.origin, preview.destination, 18);
			Lmod.polyline(
				[
					[preview.origin.latitude, preview.origin.longitude],
					...arc,
					[preview.destination.latitude, preview.destination.longitude]
				],
				{ color: '#5B9EE0', weight: 3, opacity: 0.9, dashArray: '6 6' }
			).addTo(previewLayer);
		}
	});

	onMount(() => {
		let cancelled = false;
		let observer: ResizeObserver | undefined;
		void (async () => {
			if (!container) return;
			let Leaflet: ({ default?: L } & L) | undefined;
			try {
				Leaflet = (await import('leaflet')) as unknown as { default?: L } & L;
			} catch (err) {
				console.error('RouteMapOverlay: failed to load leaflet', err);
				return;
			}
			const loadedL: L = Leaflet.default ?? Leaflet;
			if (cancelled || !container) return;
			Lmod = loadedL;

			const createdMap = loadedL
				.map(container, {
					worldCopyJump: true,
					zoomControl: true,
					attributionControl: true,
					minZoom: 3,
					maxZoom: 18
				})
				.setView([12, 108], 3);

			loadedL
				.tileLayer(
					'https://server.arcgisonline.com/ArcGIS/rest/services/Canvas/World_Dark_Gray_Base/MapServer/tile/{z}/{y}/{x}',
					{
						attribution: 'Tiles © Esri — Esri, DeLorme, NAVTEQ',
						maxZoom: 16,
						minZoom: 3,
						crossOrigin: true
					}
				)
				.addTo(createdMap);

			markersLayer = loadedL.layerGroup().addTo(createdMap);
			routesLayer = loadedL.layerGroup().addTo(createdMap);
			previewLayer = loadedL.layerGroup().addTo(createdMap);

			map = createdMap;

			const byIata = new Map(airports.map((a) => [a.iata, a]));
			const connected = new Set<string>();
			for (const r of routes) {
				connected.add(r.originIata);
				connected.add(r.destinationIata);
			}
			const used = [...connected].map((iata) => byIata.get(iata)).filter(Boolean) as Airport[];
			if (used.length > 0 && !preview) {
				const bounds = loadedL.latLngBounds(used.map((a) => [a.latitude, a.longitude]));
				createdMap.fitBounds(bounds, { padding: [40, 40], maxZoom: 6 });
			}

			const revalidate = () => createdMap.invalidateSize();
			requestAnimationFrame(() => {
				revalidate();
				requestAnimationFrame(revalidate);
			});
			setTimeout(revalidate, 250);
			if (container) {
				observer = new ResizeObserver(revalidate);
				observer.observe(container);
			}

			cleanup = () => {
				observer?.disconnect();
				createdMap.remove();
			};
		})();
		return () => {
			cancelled = true;
		};
	});

	onDestroy(() => cleanup?.());
</script>

<div class="map-wrap">
	<div class="map" bind:this={container} aria-label="Route map" role="img"></div>
</div>

<style>
	.map-wrap {
		position: absolute;
		inset: 0;
	}
	.map {
		position: absolute;
		inset: 0;
		background: var(--color-bg);
	}
</style>
