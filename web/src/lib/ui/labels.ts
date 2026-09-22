interface LabelPair {
	key: string;
	value: string;
}

interface ParsedLabels {
	pairs: LabelPair[];
	/** Segments that were not `key:value`, in the order they appeared. */
	invalid: string[];
	/** Keys that appeared more than once. Only the first occurrence is in `pairs`. */
	duplicateKeys: string[];
}

/**
 * Splits a `key:value,key:value` string, reporting what it couldn't use.
 *
 * The server rejects a malformed segment or a repeated key with a 400 (ADR
 * 0026), so the UI has to be able to say the same thing rather than quietly
 * rendering fewer chips than the URL contained — someone who hand-edits a link
 * should learn that half of it was thrown away, not wonder why their filter
 * is wider than they asked for.
 *
 * Unlike the server, the first occurrence of a repeated key is kept: this
 * feeds an editor, and dropping the whole query would leave nothing to edit.
 */
function parseLabelsStrict(value: string): ParsedLabels {
	if (value.length === 0) return { pairs: [], invalid: [], duplicateKeys: [] };

	const pairs: LabelPair[] = [];
	const invalid: string[] = [];
	const duplicateKeys: string[] = [];
	const seen = new Set<string>();

	for (const segment of value.split(',')) {
		const parts = segment.split(':');
		if (parts.length !== 2 || parts[0].length === 0 || parts[1].length === 0) {
			invalid.push(segment);
			continue;
		}

		const [key, pairValue] = parts;
		if (seen.has(key)) {
			if (!duplicateKeys.includes(key)) duplicateKeys.push(key);
			continue;
		}

		seen.add(key);
		pairs.push({ key, value: pairValue });
	}

	return { pairs, invalid, duplicateKeys };
}

/** Best-effort parse for callers that only want the usable pairs. */
function parseLabels(value: string): LabelPair[] {
	return parseLabelsStrict(value).pairs;
}

function serializeLabels(pairs: LabelPair[]): string {
	return pairs.map(({ key, value }) => `${key}:${value}`).join(',');
}

export { parseLabels, parseLabelsStrict, serializeLabels, type LabelPair, type ParsedLabels };
