import { type Observable } from "rxjs";
import type { PageRepo } from "@/repos/pages/page-repo";
import type { PageDocumentRecord, PageSaveRequest } from "@/shared/api/models";
import { KeyedStream } from "@/shared/flow/keyed-stream";
import { type Result } from "@/shared/flow/result";

export class PageDocumentService {
  private readonly stream = new KeyedStream<string, PageDocumentRecord>(
    (record) => record.id,
    (id) => this.pageRepo.getPage(id),
  );

  constructor(private readonly pageRepo: PageRepo) {}

  document(pageId: string): Observable<Result<PageDocumentRecord>> {
    return this.stream.observe(pageId);
  }

  async savePage(
    pageId: string,
    request: PageSaveRequest,
  ): Promise<PageDocumentRecord> {
    const record = await this.pageRepo.savePage(pageId, request);
    this.stream.push(record);
    return record;
  }

  /**
   * Apply a server-pushed page record into the local stream so any open
   * editor session can reconcile against the new revision. Called from
   * the realtime event handler (M7.7) when a `pageContentChanged` event
   * arrives — e.g. an MCP agent edited this page from another client.
   */
  applyExternalUpdate(record: PageDocumentRecord) {
    this.stream.push(record);
  }
}
