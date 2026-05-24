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
}
