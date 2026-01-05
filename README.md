# cine-worker
Super small and lightweight video transcoding job written in go that communicates to a master process in order to take place on a segment-based video processing mesh. Transcodes media using FFmpeg and exposes results over HTTP for adaptive streaming. Allows offloading heavy video workloads from low-power NAS and orchestration nodes. 
