import { gql } from "@apollo/client";
import { getClient } from "./core/StashService";

export type StashAudioQueueItem = { kind: "audio"; id: string };

export interface StashAudioAPI {
  version: "1.0";
  route: "/audios";
  open(id: string): void;
  play(id: string): Promise<HTMLAudioElement>;
  isAudioRoute(): boolean;
  addToQueue(ids: string[]): void;
}

const api: StashAudioAPI = {
  version: "1.0",
  route: "/audios",
  open(id) {
    window.history.pushState({}, "", `${document.baseURI}audios/${id}`);
    window.dispatchEvent(new PopStateEvent("popstate"));
  },
  async play(id) {
    const result = await getClient().query<{ findAudio: { paths: { stream: string } } }>({
      query: gql`query StashAudioPlay($id: ID!) { findAudio(id: $id) { paths { stream } } }`,
      variables: { id },
      fetchPolicy: "network-only",
    });
    const player = new Audio(result.data.findAudio.paths.stream);
    await player.play();
    return player;
  },
  isAudioRoute() { return window.location.pathname.includes("/audios"); },
  addToQueue(ids) {
    const queue = (window as Window & { StashSessionQueue?: { addAudioMany?: (ids: string[]) => unknown } }).StashSessionQueue;
    if (queue?.addAudioMany) {
      queue.addAudioMany(ids);
      return;
    }
    window.dispatchEvent(new CustomEvent("stash:audio:add-to-queue", {
      detail: ids.map((id): StashAudioQueueItem => ({ kind: "audio", id })),
    }));
  },
};

declare global { interface Window { StashAudio: StashAudioAPI } }
window.StashAudio = api;
