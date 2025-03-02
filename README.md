# qbittorrent-ban-torrentstorm
Watches the qBittorrent API for any peers with a peer ID of `-TS0008-` and bans them.

## TorrentStorm mystery explained

For [years](https://www.reddit.com/r/torrents/comments/6e8b17/what_is_torrentstorm_0008_and_why_am_i_getting/) and [years](https://www.reddit.com/r/torrents/comments/jzhvvf/whats_the_deal_with_torrentstorm_0008/) now, some peers in a torrent swarm would report a peer ID of `-TS0008-` or a client string of "TorrentStorm 0.0.0.8". TorrentStorm seems to have at one point been a torrent client, but according to this [blog post](https://www.grauw.nl/articles/bittorrent.php#TorrentStorm) has been discontinued since 2005. These peers claiming to be TorrentStorm would not be connectable and would never seed, so you would only connect to them if you had port forwarding configured. The common explanation was that it was either copyright trolls looking for dirt or some client spoofing the peer ID. But according to a dedicated Reddit user `infin1tyGR` who did some [investigation](https://www.reddit.com/r/torrents/comments/1689tuk/comment/k9sf8pj/), it seems the latter explanation is accurate and that these peers are likely [Stremio](https://github.com/Stremio), a streaming application that can use a torrent swarm as a streaming source. Stremio uses [torrent-stream](https://github.com/mafintosh/torrent-stream/blob/adc53d6ced42959ae37553e9f11aa6372e485ca0/index.js#L64) to accomplish this, which uses that `-TS0008-` peer ID we saw.

### Why try to ban these peers?

Services like Stremio and Real-Debrid explicitly never seed the content they leech from the swarm, which has a theoretically negative impact on content retention. While some peers will "permaseed" for an indeterminate amount of time, some peers will seed on a ratio basis (meaning that they will upload maybe 2-3 times as much data as they download). These services that leech from the swarm will exhaust the allotted upload ratio of these peers which means that the content will not be further uploaded and shared by the peer seeding on a ratio basis.

### Tit for tat - Stremio's response

It also seems that the Stremio devs saw this same Reddit post, because not a month after it being published a commit was merged in their backend that allows the user to [spoof the peer ID to more popular clients](https://github.com/Stremio/enginefs/pull/30/commits/c62b8b7d8ea02d23318fea1f5a75d601e5c81a4c), an evasion technique not used previously. My theory is that until that post was made, no one had really figured out it who TorrentStorm peers were and that there was no reason to obfuscate their identity by spoofing the peer ID. But now that Stremio has been identified as the likely source of these peers, they are spoofing the ID to evade bans by seeds in the swarm. 

### So if Stremio isn't using the TorrentStorm peer ID anymore, doesn't that make this project useless?

Not necessarily. The aforementioned change allows the user to spoof the peer ID, but it doesn't seem to be a default option and thus plenty of TorrentStorm peers are visible in swarms. Admittedly, spoofing the peer ID would avoid being blocked by this particular project, but it isn't impossible. Since the peer IDs that can be spoofed are hard coded, one could theoretically block those peers as well but that would include plenty of friendly fire towards otherwise innocent peers who happen to be using particular versions of the most popular torrent clients.

In any case, I thought it'd be fun to put together some information on a neat little internet mystery that'd been on my mind for a while and make a fun project out of it.