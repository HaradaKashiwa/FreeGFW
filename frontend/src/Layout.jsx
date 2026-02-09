import { useEffect, useCallback, useState } from "react"
import { useGetConfigs, useSetTitle } from "./apis/config"
import { useNavigate } from "react-router-dom"
import { IoChevronForward, IoSettings, IoPencil } from "react-icons/io5"
import { Modal } from "./components/Modal"
import { SettingsCard } from "./components/SettingsCard"

export default function Layout({ children }) {
    const { data: configs, loaded, refresh } = useGetConfigs()
    const [editable, setEditable] = useState(false)
    const navigate = useNavigate()
    const { trigger: setTitle, loading: setTitleLoading, error: setTitleError } = useSetTitle()
    useEffect(() => {
        if (!loaded) return
        if (!configs?.inited) navigate('/start')
        document.title = configs?.title || 'FreeGFW'
    }, [configs, loaded])

    return (
        <div className='px-2'>
            <div className='py-4 max-w-4xl mx-auto flex items-center justify-between gap-4'>
                <div className='flex items-center gap-4'>
                    <div className='text-2xl'>
                        {!editable && <div className='flex items-center gap-2 cursor-pointer' onClick={() => setEditable(true)}><span>{configs?.title || 'FreeGFW'}</span> <IoPencil /></div>}
                        {editable && <input autoFocus defaultValue={configs?.title || 'FreeGFW'} onBlur={async (e) => {
                            if (e.target.value !== configs?.title) {
                                await setTitle({
                                    title: e.target.value
                                })
                                await refresh()
                            }
                            setEditable(false)
                        }} />}
                    </div>
                </div>
                <div className='flex items-center gap-4 text-sm'>
                    <Modal
                        content={(
                            <SettingsCard />
                        )}
                        title={'系统设置'}
                    >
                        <div className='cursor-pointer p-2 border rounded-full text-xl'><IoSettings /></div>
                    </Modal>
                </div>
            </div>
            <div className='max-w-4xl mx-auto'>
                {children}
            </div>
        </div>
    )
}