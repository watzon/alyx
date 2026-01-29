export interface MetricDataPoint {
	timestamp: number;
	httpRequests: number;
	httpRequestsByStatus: Record<string, number>;
	dbConnections: { open: number; inUse: number };
	memoryBytes?: number;
	goroutines?: number;
}

const MAX_DATA_POINTS = 120;

function createMetricsStore() {
	let dataPoints = $state<MetricDataPoint[]>([]);

	function addDataPoint(point: MetricDataPoint): void {
		dataPoints = [...dataPoints, point];

		if (dataPoints.length > MAX_DATA_POINTS) {
			dataPoints = dataPoints.slice(1);
		}
	}

	function getDataPoints(): MetricDataPoint[] {
		return dataPoints;
	}

	function getLatest(): MetricDataPoint | undefined {
		return dataPoints[dataPoints.length - 1];
	}

	return {
		get dataPoints() {
			return dataPoints;
		},
		addDataPoint,
		getDataPoints,
		getLatest
	};
}

export const metricsStore = createMetricsStore();
