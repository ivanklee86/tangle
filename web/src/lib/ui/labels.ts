interface LabelPair {
	key: string;
	value: string;
}

function parseLabels(value: string): LabelPair[] {
	if (value.length === 0) return [];
	return value
		.split(',')
		.map((pair) => pair.split(':'))
		.filter((parts): parts is [string, string] => parts.length === 2)
		.map(([key, value]) => ({ key, value }));
}

function serializeLabels(pairs: LabelPair[]): string {
	return pairs.map(({ key, value }) => `${key}:${value}`).join(',');
}

export { parseLabels, serializeLabels, type LabelPair };
