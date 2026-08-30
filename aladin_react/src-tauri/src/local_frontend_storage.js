const storageMigration = {
  marker: "aladin.local_frontend.migrated.v1",
  collect(storage) {
    const entries = {};
    for (let i = 0; i < storage.length; i++) {
      const key = storage.key(i);
      if (key && key.startsWith("aladin.") && key !== this.marker) {
        entries[key] = storage.getItem(key);
      }
    }
    return entries;
  },
  restore(storage, entries) {
    // Never resurrect a legacy login after the user logs out on the new origin.
    if (storage.getItem(this.marker)) return;
    for (const [key, value] of Object.entries(entries)) {
      if (key.startsWith("aladin.") && key !== this.marker && typeof value === "string"
          && storage.getItem(key) === null) {
        storage.setItem(key, value);
      }
    }
    storage.setItem(this.marker, "1");
  },
};
