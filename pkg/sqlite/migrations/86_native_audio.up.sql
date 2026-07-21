CREATE TABLE `audio_files` (
  `file_id` integer not null primary key REFERENCES `files`(`id`) ON DELETE CASCADE,
  `format` varchar(255) not null,
  `duration` float not null,
  `audio_codec` varchar(255) not null,
  `bit_rate` bigint not null,
  `sample_rate` integer not null,
  `channels` integer not null,
  `bit_depth` integer not null default 0
);

CREATE TABLE `audios` (
  `id` integer not null primary key autoincrement,
  `title` varchar(255),
  `album` varchar(255),
  `grouping` varchar(255),
  `details` text,
  `audience` varchar(255),
  `content_type` varchar(255),
  `rating` tinyint,
  `organized` boolean not null default 0,
  `resume_time` float not null default 0,
  `play_duration` float not null default 0,
  `play_count` integer not null default 0,
  `last_played_at` datetime,
  `cover_blob` varchar(255) REFERENCES `blobs`(`checksum`),
  `created_at` datetime not null,
  `updated_at` datetime not null
);

CREATE TABLE `audios_files` (
  `audio_id` integer not null REFERENCES `audios`(`id`) ON DELETE CASCADE,
  `file_id` integer not null REFERENCES `files`(`id`) ON DELETE CASCADE,
  `primary` boolean not null default 0,
  PRIMARY KEY (`audio_id`, `file_id`)
);

CREATE UNIQUE INDEX `audios_files_primary_unique` ON `audios_files` (`audio_id`) WHERE `primary` = 1;
CREATE INDEX `audios_files_file_id` ON `audios_files` (`file_id`);

CREATE TABLE `groups_audios` (
  `group_id` integer not null REFERENCES `groups`(`id`) ON DELETE CASCADE,
  `audio_id` integer not null REFERENCES `audios`(`id`) ON DELETE CASCADE,
  PRIMARY KEY (`group_id`, `audio_id`)
);

CREATE TABLE `audios_tags` (
  `audio_id` integer not null REFERENCES `audios`(`id`) ON DELETE CASCADE,
  `tag_id` integer not null REFERENCES `tags`(`id`) ON DELETE CASCADE,
  `source` varchar(32) not null default 'manual',
  PRIMARY KEY (`audio_id`, `tag_id`)
);

CREATE TABLE `audio_genres` (
  `audio_id` integer not null REFERENCES `audios`(`id`) ON DELETE CASCADE,
  `value` varchar(255) not null,
  `normalized_value` varchar(255) not null,
  PRIMARY KEY (`audio_id`, `normalized_value`)
);

CREATE INDEX `audio_genres_normalized_value` ON `audio_genres` (`normalized_value`);

CREATE TABLE `audio_descriptors` (
  `audio_id` integer not null REFERENCES `audios`(`id`) ON DELETE CASCADE,
  `value` varchar(255) not null,
  `normalized_value` varchar(255) not null,
  PRIMARY KEY (`audio_id`, `normalized_value`)
);

CREATE INDEX `audio_descriptors_normalized_value` ON `audio_descriptors` (`normalized_value`);

CREATE TABLE `audio_custom_fields` (
  `audio_id` integer not null REFERENCES `audios`(`id`) ON DELETE CASCADE,
  `field` varchar(255) not null,
  `value` text,
  PRIMARY KEY (`audio_id`, `field`)
);

CREATE INDEX `audios_title` ON `audios` (`title`);
CREATE INDEX `audios_album` ON `audios` (`album`);
CREATE INDEX `audios_rating` ON `audios` (`rating`);
CREATE INDEX `audios_updated_at` ON `audios` (`updated_at`);
