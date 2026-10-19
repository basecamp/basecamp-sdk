package com.basecamp.sdk.services

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

class CampfireRoomUrlTest {

    @Test
    fun readsTheCampfireIdOutOfARoomUrl() {
        assertEquals(1069479600L, campfireIdFromRoomUrl("https://3.basecampapi.com/999/buckets/2085958600/chats/1069479600.json"))
        assertEquals(1069479600L, campfireIdFromRoomUrl("https://3.basecampapi.com/999/buckets/2085958600/chats/1069479600"))
        assertEquals(1069479600L, campfireIdFromRoomUrl("https://3.basecampapi.com/999/chats/1069479600.json"))
        assertEquals(1069479600L, campfireIdFromRoomUrl("/999/chats/1069479600.json"))
        assertEquals(1069479600L, campfireIdFromRoomUrl("https://3.basecampapi.com/999/chats/1069479600.json?x=1#frag"))
    }

    @Test
    fun aUrlThatNamesNoCampfireYieldsNone() {
        assertNull(campfireIdFromRoomUrl("https://3.basecampapi.com/999/buckets/1/chats/5/lines.json"))
        assertNull(campfireIdFromRoomUrl("https://3.basecampapi.com/999/buckets/1/todolists/5.json"))
        assertNull(campfireIdFromRoomUrl("https://3.basecampapi.com/999/buckets/1/chats/abc.json"))
        assertNull(campfireIdFromRoomUrl("https://3.basecampapi.com/999/buckets/1/chats/0.json"))
        assertNull(campfireIdFromRoomUrl("https://chats/5"))
        assertNull(campfireIdFromRoomUrl(""))
    }
}
