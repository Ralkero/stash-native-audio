import React, { useEffect, useState } from "react";
import { gql, useQuery } from "@apollo/client";
import { Button, Card, Col, Form, Row } from "react-bootstrap";
import { Link } from "react-router-dom";
import { LoadingIndicator } from "../Shared/LoadingIndicator";

const QUERY = gql`
  query NativeAudioBrowse($filter: FindFilterType) {
    findAudios(filter: $filter) {
      count duration filesize
      audios {
        id title album audience content_type rating100
        authors { id name }
        genres
        files { id duration audio_codec bitrate: bit_rate }
        paths { stream cover }
      }
    }
  }
`;

type AudioSummary = {
  id: string; title?: string; album?: string; audience?: string; content_type?: string;
  rating100?: number; authors: { id: string; name: string }[]; genres: string[];
  files: { id: string; duration: number; audio_codec: string; bitrate: number }[];
  paths: { stream: string; cover?: string };
};

function duration(seconds: number) {
  const minutes = Math.floor(seconds / 60);
  return `${minutes}:${Math.floor(seconds % 60).toString().padStart(2, "0")}`;
}

export const AudioBrowse: React.FC = () => {
  const [q, setQ] = useState("");
  const [sort, setSort] = useState("title");
  const [direction, setDirection] = useState("ASC");
  const [page, setPage] = useState(1);
  const { data, loading, error } = useQuery<{ findAudios: { count: number; audios: AudioSummary[] } }>(QUERY, {
    variables: { filter: { q: q || undefined, sort, direction, page, per_page: 24 } },
  });
  useEffect(() => setPage(1), [q, sort, direction]);
  const pages = Math.max(1, Math.ceil((data?.findAudios.count ?? 0) / 24));
  return <div className="audio-library px-2 py-3" data-stash-vocabulary-preserve="true">
    <div className="audio-toolbar p-3 mb-3">
      <div><h2>Audio Library</h2><span>{data?.findAudios.count ?? 0} indexed audio items</span></div>
      <Form.Control value={q} onChange={(e) => setQ(e.currentTarget.value)} placeholder="Search title, album, or description" />
      <Form.Control as="select" value={sort} onChange={(e) => setSort(e.currentTarget.value)}>
        <option value="title">Title</option><option value="author">Author</option><option value="album">Album</option>
        <option value="genre">Genre</option><option value="descriptor">Descriptor</option><option value="audience">Audience</option>
        <option value="duration">Duration</option><option value="rating">Rating</option><option value="path">Path</option>
        <option value="created_at">Date added</option><option value="updated_at">Recently updated</option><option value="play_count">Play count</option>
      </Form.Control>
      <Button variant="secondary" onClick={() => setDirection(direction === "ASC" ? "DESC" : "ASC")}>{direction}</Button>
    </div>
    {loading && <LoadingIndicator />}
    {error && <div className="alert alert-danger">{error.message}</div>}
    <Row>
      {data?.findAudios.audios.map((audio) => <Col key={audio.id} xs={12} md={6} xl={4} className="mb-3">
        <Card className="audio-card h-100">
          <Card.Body>
            <div className="d-flex justify-content-between"><div>
              <Card.Title><Link to={`/audios/${audio.id}`}>{audio.title || "Untitled audio"}</Link></Card.Title>
              <div className="audio-muted">{audio.authors.map((x) => x.name).join(", ") || "Unknown author"}</div>
              <div className="audio-muted">{audio.album || "No album"}</div>
            </div>{audio.files[0] && <span>{duration(audio.files[0].duration)}</span>}</div>
            <audio className="w-100 mt-3" controls preload="metadata" src={audio.paths.stream} />
            <div className="mt-2">{audio.genres.slice(0, 5).map((genre) => <span className="audio-chip" key={genre}>{genre}</span>)}</div>
          </Card.Body>
        </Card>
      </Col>)}
    </Row>
    {pages > 1 && <div className="d-flex justify-content-center align-items-center mt-3">
      <Button variant="secondary" disabled={page <= 1} onClick={() => setPage(page - 1)}>Previous</Button>
      <span className="mx-3">Page {page} of {pages}</span>
      <Button variant="secondary" disabled={page >= pages} onClick={() => setPage(page + 1)}>Next</Button>
    </div>}
  </div>;
};
