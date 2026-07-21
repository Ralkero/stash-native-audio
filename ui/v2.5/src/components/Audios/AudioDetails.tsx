import React, { useEffect, useRef, useState } from "react";
import { gql, useMutation, useQuery } from "@apollo/client";
import { Button, Form } from "react-bootstrap";
import { RouteComponentProps } from "react-router-dom";
import { LoadingIndicator } from "../Shared/LoadingIndicator";

const QUERY = gql`query NativeAudioDetail($id: ID!) { findAudio(id: $id) {
  id title album grouping details audience content_type rating100 organized resume_time has_cover
  authors { id name } tags { id name } genres descriptors custom_fields
  files { id path duration audio_codec sample_rate channels bit_depth }
  paths { stream cover }
} }`;
const UPDATE = gql`mutation NativeAudioUpdate($input: AudioUpdateInput!) { audioUpdate(input: $input) {
  id title album grouping details audience content_type rating100 organized genres descriptors
} }`;
const INCREMENT = gql`mutation NativeAudioPlayed($id: ID!) { audioIncrementPlayCount(id: $id) }`;
const SAVE_ACTIVITY = gql`mutation NativeAudioActivity($id: ID!, $resume: Float, $duration: Float) { audioSaveActivity(id: $id, resume_time: $resume, playDuration: $duration) }`;

type Detail = { id: string; title?: string; album?: string; grouping?: string; details?: string; audience?: string;
  content_type?: string; rating100?: number; organized: boolean; resume_time: number; has_cover: boolean; genres: string[]; descriptors: string[];
  authors: {id:string;name:string}[]; tags:{id:string;name:string}[]; files:{id:string;path:string;duration:number;audio_codec:string;sample_rate:number;channels:number;bit_depth:number}[];
  paths:{stream:string;cover?:string}; custom_fields: Record<string, unknown> };

export const AudioDetails: React.FC<RouteComponentProps<{id:string}>> = ({ match }) => {
  const { data, loading, error } = useQuery<{findAudio: Detail}>(QUERY, { variables: { id: match.params.id } });
  const [update, updateState] = useMutation(UPDATE, { refetchQueries: [{ query: QUERY, variables: { id: match.params.id } }] });
  const [incrementPlay] = useMutation(INCREMENT);
  const [saveActivity] = useMutation(SAVE_ACTIVITY);
  const [form, setForm] = useState<Record<string, string>>({});
  const [loop, setLoop] = useState(false);
  const playStarted = useRef(0);
  const playCounted = useRef(false);
  const lastResumeSave = useRef(0);
  useEffect(() => { const a = data?.findAudio; if (a) setForm({ title:a.title||"", album:a.album||"", grouping:a.grouping||"", details:a.details||"", audience:a.audience||"", content_type:a.content_type||"", genres:a.genres.join("; "), descriptors:a.descriptors.join("; "), rating100:String(a.rating100||"") }); }, [data]);
  if (loading) return <LoadingIndicator />;
  if (error || !data?.findAudio) return <div className="alert alert-danger">{error?.message || "Audio not found"}</div>;
  const a = data.findAudio;
  const field = (name:string, label:string, area=false) => <Form.Group><Form.Label>{label}</Form.Label>{area ? <Form.Control as="textarea" rows={3} value={form[name]||""} onChange={(e)=>setForm({...form,[name]:e.currentTarget.value})}/> : <Form.Control value={form[name]||""} onChange={(e)=>setForm({...form,[name]:e.currentTarget.value})}/>}</Form.Group>;
  const save = () => update({ variables: { input: { id:a.id, title:form.title, album:form.album, grouping:form.grouping, details:form.details, audience:form.audience, content_type:form.content_type, rating100:form.rating100 ? Number(form.rating100):null, genres:form.genres.split(";").map(x=>x.trim()).filter(Boolean), descriptors:form.descriptors.split(";").map(x=>x.trim()).filter(Boolean) } } });
  const recordActivity = (player: HTMLAudioElement, includeDuration: boolean) => {
    const now = Date.now();
    const played = includeDuration && playStarted.current ? Math.max(0, (now - playStarted.current) / 1000) : undefined;
    if (includeDuration) playStarted.current = player.paused ? 0 : now;
    saveActivity({ variables: { id: a.id, resume: player.ended ? 0 : player.currentTime, duration: played } }).catch(() => undefined);
  };
  return <div className="audio-library py-3" data-stash-vocabulary-preserve="true"><div className="audio-detail p-4">
    <div className="d-flex flex-wrap justify-content-between"><div><h2>{a.title || "Untitled audio"}</h2><div className="audio-muted">{a.authors.map(x=>x.name).join(", ") || "Unknown author"}</div></div>{a.has_cover && <img className="audio-cover" src={a.paths.cover} alt="Cover"/>}</div>
    <audio className="w-100 my-4" controls preload="metadata" src={a.paths.stream} loop={loop} autoPlay={new URLSearchParams(location.search).get("autoplay") === "true"}
      onLoadedMetadata={(e)=>{ if (a.resume_time > 0 && a.resume_time < e.currentTarget.duration - 2) e.currentTarget.currentTime=a.resume_time; }}
      onPlay={()=>{ playStarted.current=Date.now(); if(!playCounted.current){playCounted.current=true; incrementPlay({variables:{id:a.id}});} }}
      onTimeUpdate={(e)=>{ if(Date.now()-lastResumeSave.current>5000){lastResumeSave.current=Date.now(); recordActivity(e.currentTarget,false);} }}
      onPause={(e)=>recordActivity(e.currentTarget,true)} onEnded={(e)=>recordActivity(e.currentTarget,true)}/>
    <Form.Check type="checkbox" label="Loop audio" checked={loop} onChange={(e)=>setLoop(e.currentTarget.checked)}/>
    <Form>{field("title","Title")}{field("album","Album")}{field("grouping","Grouping")}{field("audience","Audience")}{field("content_type","Content type")}{field("genres","Genres (semicolon-separated)")}{field("descriptors","Descriptors (semicolon-separated)")}{field("details","Description",true)}{field("rating100","Rating (1–100)")}
      <Button onClick={save} disabled={updateState.loading}>{updateState.loading ? "Saving…" : "Save metadata"}</Button>
    </Form>
    <div className="mt-3"><strong>Shared Tags:</strong> {a.tags.map((tag) => tag.name).join(", ") || "None"}</div>
    <hr/><h4>Technical details</h4>{a.files.map(f=><div key={f.id} className="audio-muted">{f.path} · {f.audio_codec} · {f.sample_rate} Hz · {f.channels} channels{f.bit_depth ? ` · ${f.bit_depth}-bit`:""}</div>)}
  </div></div>;
};
