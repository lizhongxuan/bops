function toStringArray(value: unknown): string[] {
  if (Array.isArray(value)) {
    return value.filter((item): item is string => typeof item === "string");
  }
  if (typeof value === "string") {
    return [value];
  }
  return [];
}

export function normalizeQuestions(value: unknown): string[] {
  const items = toStringArray(value);
  const seen = new Set<string>();
  const result: string[] = [];
  for (const item of items) {
    const trimmed = item.trim();
    if (!trimmed || seen.has(trimmed)) {
      continue;
    }
    seen.add(trimmed);
    result.push(trimmed);
  }
  return result;
}

export function resolveQuestions(primary: unknown, fallback: unknown, limit = 6): string[] {
  const primaryList = normalizeQuestions(primary);
  const fallbackList = normalizeQuestions(fallback);
  const source = primaryList.length ? primaryList : fallbackList;
  return source.slice(0, limit);
}
