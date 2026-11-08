"use client";

import { useEffect, useState } from "react";
import { getSample } from "./api/notes";

// Whether the sample lecture can be opened. False until known, and when none is loaded (404) or the API can't be
// reached: a link to it only shows once it would work. `ask` false skips the request.
export function useHasSample(ask: boolean): boolean {
  const [sample, setSample] = useState(false);

  useEffect(() => {
    if (!ask) return;
    const abort = new AbortController();
    getSample(abort.signal).then((job) => setSample(job !== null), () => {});
    return () => abort.abort();
  }, [ask]);

  return sample;
}
