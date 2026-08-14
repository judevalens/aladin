package dawn.system.anchor.services.data

import dawn.system.anchor.services.data.Entry

/**
 * Repository for journal [Entry] entities. Inherits the seq-guarded [sync] from
 * [Repository]; the entry-specific read/query contract (e.g. a collection flow and/or a
 * per-entity flow) is declared here when this repository is implemented.
 */
interface EntryRepository : Repository<Entry> {
    // Entry-specific read/query methods go here.
}
