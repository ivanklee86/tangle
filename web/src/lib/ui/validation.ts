const LABEL_FORMAT: RegExp = /^[^:,]+:[^:,]+(,[^:,]+:[^:,]+)*$/;

function isValidLabelFormat(value: string): boolean {
	return value.length === 0 || LABEL_FORMAT.test(value);
}

export { isValidLabelFormat };
