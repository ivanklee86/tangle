import { parseLabelsStrict } from '$lib/ui/labels';

const LABEL_FORMAT: RegExp = /^[^:,]+:[^:,]+(,[^:,]+:[^:,]+)*$/;

/** Whether every segment is a well-formed `key:value`. Empty is valid. */
function isValidLabelFormat(value: string): boolean {
	return value.length === 0 || LABEL_FORMAT.test(value);
}

/**
 * Whether a label string is one the API will accept: well formed, and with
 * each key appearing at most once.
 *
 * Uniqueness is not a style preference — the server builds a single Kubernetes
 * selector from these and rejects a repeated key with a 400 (ADR 0026),
 * because `env=test,env=prod` can never match. Checking it here keeps the UI
 * from offering a query the API will refuse.
 */
function isValidLabelQuery(value: string): boolean {
	if (!isValidLabelFormat(value)) return false;
	return parseLabelsStrict(value).duplicateKeys.length === 0;
}

/** Keys appearing more than once, for an error message that can name them. */
function duplicateLabelKeys(value: string): string[] {
	return parseLabelsStrict(value).duplicateKeys;
}

export { isValidLabelFormat, isValidLabelQuery, duplicateLabelKeys };
