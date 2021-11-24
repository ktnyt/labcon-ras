import { useEffect, useState } from "react";

import axios from "axios";

import { Button, Card, CardContent, Grid, Typography } from "@mui/material";

import { useInterval } from "./hooks/useInterval";

const client = axios.create({
  baseURL: process.env.REACT_APP_OPERATOR_ADDR || "http://localhost:5000",
  headers: { "Content-Type": "application/json" },
  transformRequest: (data) => JSON.stringify(data),
  transformResponse: (data) => JSON.parse(data),
});

type Arm = {
  status: string;
  spot: boolean;
};

type StationProps = {
  name: string;
  arm: Arm;
};

const Station = ({ name, arm }: StationProps) => {
  const [state, setState] = useState<boolean[]>([]);
  const [status, setStatus] = useState("");
  const station = Number(name.replace("station", ""));

  useInterval(async () => {
    const res = await client.get<boolean[]>(`/driver/${name}/state`);
    setState(res.data);
  }, 1000);

  useInterval(async () => {
    const res = await client.get<string>(`/driver/${name}/status`);
    setStatus(res.data);
  }, 1000);

  const dispatch = async (name: string, spot: number) => {
    const arg = { station, spot };
    const params = { name, arg };
    await client.post(`/driver/arm/operation`, params).catch(() => {});
  };

  return (
    <Card>
      <CardContent>
        <Grid container direction="column" spacing={2}>
          <Grid item>
            <Typography variant="h5">{name}</Typography>
          </Grid>
          <Grid item>
            <Typography variant="body2">Status: {status}</Typography>
          </Grid>
          {state.map((spot, index) => (
            <Grid
              key={index}
              container
              item
              spacing={2}
              justifyContent="space-between"
              alignItems="center"
            >
              <Grid item xs={6}>
                <Typography variant="body1">
                  {`Spot ${index}`}: {`${spot}`}
                </Typography>
              </Grid>

              <Grid container item spacing={2} xs={6}>
                <Grid item>
                  <Button
                    variant="outlined"
                    color="primary"
                    disabled={!spot || arm.status !== "idle"}
                    onClick={() => dispatch("take", index)}
                  >
                    Take
                  </Button>
                </Grid>

                <Grid item>
                  <Button
                    variant="outlined"
                    color="secondary"
                    disabled={!arm.spot || arm.status !== "idle"}
                    onClick={() => dispatch("put", index)}
                  >
                    Put
                  </Button>
                </Grid>
              </Grid>
            </Grid>
          ))}
        </Grid>
      </CardContent>
    </Card>
  );
};

type ContentProps = {
  stations: string[];
};

const Content = ({ stations }: ContentProps) => {
  const [spot, setSpot] = useState(false);
  const [status, setStatus] = useState("");

  useInterval(async () => {
    const res = await client.get<boolean>("/driver/arm/state");
    setSpot(res.data);
  }, 1000);

  useInterval(async () => {
    const res = await client.get<string>("/driver/arm/status");
    setStatus(res.data);
  }, 1000);

  return (
    <Grid container spacing={4}>
      <Grid item>
        <Card>
          <CardContent>
            <Grid container direction="column" spacing={2}>
              <Grid item>
                <Typography variant="h5">arm</Typography>
              </Grid>
              <Grid item>
                <Typography variant="body2">Status: {status}</Typography>
              </Grid>
              <Grid item>
                <Typography variant="body2">Spot: {`${spot}`}</Typography>
              </Grid>
              <Grid item>
                <Button
                  variant="outlined"
                  color="error"
                  disabled={status === "busy"}
                  onClick={() => {
                    client
                      .post(`/driver/arm/operation`, { name: "reboot" })
                      .catch(() => {});
                  }}
                >
                  Reboot
                </Button>
              </Grid>
            </Grid>
          </CardContent>
        </Card>
      </Grid>

      {stations.map((name, index) => (
        <Grid item key={index}>
          <Station name={name} arm={{ status, spot }} />
        </Grid>
      ))}
    </Grid>
  );
};

const App = () => {
  const [stations, setStations] = useState<string[]>([]);
  useEffect(() => {
    client
      .get<string[]>("/driver")
      .then((res) =>
        setStations(res.data.filter((name) => name.startsWith("station")))
      );
  }, []);
  return <Content stations={stations} />;
};

export default App;
