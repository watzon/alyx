/**
 * Prometheus metrics parser for Alyx admin dashboard
 * Parses specific metrics from the /metrics endpoint
 */

export interface PrometheusMetrics {
	httpRequestsTotal: number;
	httpRequestsByStatus: Record<string, number>; // { "200": 150, "404": 5 }
	dbConnectionsOpen: number;
	dbConnectionsInUse: number;
}

/**
 * Parse Prometheus text format metrics
 * Format: metric_name{label="value"} numeric_value
 */
export function parsePrometheusMetrics(text: string): PrometheusMetrics {
	const lines = text.split('\n');
	
	const metrics: PrometheusMetrics = {
		httpRequestsTotal: 0,
		httpRequestsByStatus: {},
		dbConnectionsOpen: 0,
		dbConnectionsInUse: 0
	};

	for (const line of lines) {
		const trimmed = line.trim();
		
		// Skip comments and empty lines
		if (!trimmed || trimmed.startsWith('#')) {
			continue;
		}

		// Parse alyx_http_requests_total{method="GET",path="/api/users",status="200"} 150
		if (trimmed.startsWith('alyx_http_requests_total{')) {
			const match = trimmed.match(/alyx_http_requests_total\{[^}]*status="(\d+)"[^}]*\}\s+(\d+(?:\.\d+)?)/);
			if (match) {
				const status = match[1];
				const value = parseFloat(match[2]);
				metrics.httpRequestsByStatus[status] = (metrics.httpRequestsByStatus[status] || 0) + value;
				metrics.httpRequestsTotal += value;
			}
		}
		
		// Parse alyx_db_connections_open 5
		else if (trimmed.startsWith('alyx_db_connections_open ')) {
			const match = trimmed.match(/alyx_db_connections_open\s+(\d+(?:\.\d+)?)/);
			if (match) {
				metrics.dbConnectionsOpen = parseFloat(match[1]);
			}
		}
		
		// Parse alyx_db_connections_in_use 2
		else if (trimmed.startsWith('alyx_db_connections_in_use ')) {
			const match = trimmed.match(/alyx_db_connections_in_use\s+(\d+(?:\.\d+)?)/);
			if (match) {
				metrics.dbConnectionsInUse = parseFloat(match[1]);
			}
		}
	}

	return metrics;
}

/**
 * Fetch and parse metrics from the /metrics endpoint
 */
export async function fetchMetrics(): Promise<PrometheusMetrics> {
	const response = await fetch('/metrics', { credentials: 'same-origin' });
	
	if (!response.ok) {
		throw new Error(`Failed to fetch metrics: ${response.statusText}`);
	}
	
	const text = await response.text();
	return parsePrometheusMetrics(text);
}
