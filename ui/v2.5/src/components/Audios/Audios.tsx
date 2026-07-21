import React from "react";
import { Route, Switch } from "react-router-dom";
import { AudioBrowse } from "./AudioBrowse";
import { AudioDetails } from "./AudioDetails";
import "./audios.scss";

const Audios: React.FC = () => (
  <div data-stash-vocabulary-preserve="true">
    <Switch>
      <Route exact path="/audios" component={AudioBrowse} />
      <Route path="/audios/:id" component={AudioDetails} />
    </Switch>
  </div>
);

export default Audios;
