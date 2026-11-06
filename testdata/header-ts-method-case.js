import { request } from 'node:http';

export interface Chunk {
	text: string;
}

export class OllamaClient {
	constructor(private host: string) {}

	async stream(prompt: string): Promise<void> {
		request(this.host + prompt);
	}

	/** Handle one NDJSON line. */
	private handle(line: string): Chunk {
		return { text: line };
	}
}
